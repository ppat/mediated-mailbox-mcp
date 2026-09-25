//go:build integration

package lease_test

import (
	"context"
	"errors"
	"math"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The two series the runaway rule reads, the count of provider request cost and the account's hard
// cap beside it. The provider adapters emit both where each request is sent, so nothing in this
// package declares them.
const (
	requestCostName = "mediated_mailbox_provider_request_cost_total"
	hardCapName     = "mediated_mailbox_provider_hard_cap"
)

// reloadFailedName is the policy loader's reload-failure series, which the policy reload rule reads.
const reloadFailedName = "mediated_mailbox_policyload_reload_failed"

// series is one gathered sample, by metric name and account.
type series struct {
	name, account, other string
}

// gathered returns every sample reg holds, keyed by name, account and any other label's value.
func gathered(t *testing.T, reg prometheus.Gatherer) map[series]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[series]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			s := series{name: f.GetName()}
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" {
					s.account = l.GetValue()
				} else {
					s.other = l.GetValue()
				}
			}
			out[s] = value(m)
		}
	}
	return out
}

func value(m *dto.Metric) float64 {
	switch {
	case m.GetGauge() != nil:
		return m.GetGauge().GetValue()
	case m.GetCounter() != nil:
		return m.GetCounter().GetValue()
	default:
		return math.NaN()
	}
}

// Each process's Limiter counts the tokens leased to it by class and the throttles it received by
// scope, and sets the rate it last read or wrote (ADR-0024, ADR-0076).
func TestTheLimiterEmitsItsProcesssSeries(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	reg := prometheus.NewRegistry()
	metrics, err := lease.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	l := lease.New(spenders(t), ceiling, metrics)
	for range 2 {
		if _, err := l.Acquire(t.Context(), account, core.Batch, 10); err != nil {
			t.Fatal(err)
		}
	}
	signal := mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true, Scope: mail.ScopePerUser}
	if err := l.Throttled(t.Context(), account, signal); err != nil {
		t.Fatal(err)
	}
	want := map[series]float64{
		{"mediated_mailbox_ratelimit_rate", account, ""}:                25,
		{"mediated_mailbox_ratelimit_granted_total", account, "batch"}:  20,
		{"mediated_mailbox_ratelimit_throttles_total", account, "user"}: 1,
	}
	if diff := cmp.Diff(want, gathered(t, reg), compare.Options); diff != "" {
		t.Errorf("series (-want +got):\n%s", diff)
	}
	if _, err := lease.NewMetrics(reg); err == nil {
		t.Error("the series registered twice on one registry without an error")
	}
}

// The collector reports each account's rate, floor, bucket level and the seconds since the last ask
// and the last grant from its rate state, by the database's clock. An account that has never spent
// is reported idle at the target, with no ask and no grant, which read as infinitely long ago, so the
// stall rule never reads a waiting account as healthy (ADR-0077).
func TestTheCollectorReportsEachAccountsRateState(t *testing.T) {
	conn := superuser(t)
	spent := newAccount(t, conn)
	idle := spent + "-idle"
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", idle)
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, bucket_level, class_asked_at, last_granted_at)
		VALUES ($1, 12.5, 50, 80, 7, ARRAY[NULL, now() - interval '2 seconds', now() - interval '30 seconds'], now() - interval '10 seconds')`, spent)

	c := lease.NewCollector(spenders(t), func(context.Context) ([]lease.Account, error) {
		return []lease.Account{{ID: spent, Ceiling: ceiling}, {ID: idle, Ceiling: ceiling}}, nil
	})
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	got := gathered(t, reg)

	exact := map[series]float64{
		{"mediated_mailbox_ratelimit_account_rate", spent, ""}:               12.5,
		{"mediated_mailbox_ratelimit_account_floor", spent, ""}:              5,
		{"mediated_mailbox_ratelimit_account_floor", idle, ""}:               5,
		{"mediated_mailbox_ratelimit_account_bucket_level", spent, ""}:       7,
		{"mediated_mailbox_ratelimit_account_rate", idle, ""}:                50,
		{"mediated_mailbox_ratelimit_account_bucket_level", idle, ""}:        0,
		{"mediated_mailbox_ratelimit_account_seconds_since_ask", idle, ""}:   math.Inf(1),
		{"mediated_mailbox_ratelimit_account_seconds_since_grant", idle, ""}: math.Inf(1),
	}
	for s, want := range exact {
		if v, ok := got[s]; !ok || v != want {
			t.Errorf("%s{account=%q} = %v (present %t), want %v", s.name, s.account, v, ok, want)
		}
	}
	if v := got[series{"mediated_mailbox_ratelimit_account_seconds_since_ask", spent, ""}]; v < 1.5 || v > 10 {
		t.Errorf("seconds since the latest ask = %v, want about 2, from the latest of the classes' instants", v)
	}
	if v := got[series{"mediated_mailbox_ratelimit_account_seconds_since_grant", spent, ""}]; v < 9.5 || v > 20 {
		t.Errorf("seconds since the last grant = %v, want about 10", v)
	}
}

// A rate held at a floor that a four-byte float cannot hold exactly is stored rounded, and the
// collector emits the floor at that same precision, so the collapse rule's rate <= floor holds. A
// ceiling of 6 gives a floor of 0.3, which the stored rate rounds up (ADR-0077).
func TestAFloorRateReadsAsAtTheFloor(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	const odd = 6
	l := lease.New(spenders(t), odd, nil)
	signal := mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true, Scope: mail.ScopePerUser}
	// Halving from the target of 3 reaches the floor of 0.3 within four throttles.
	if _, err := l.Acquire(t.Context(), account, core.Batch, 0.1); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if err := l.Throttled(t.Context(), account, signal); err != nil {
			t.Fatal(err)
		}
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(lease.NewCollector(spenders(t), func(context.Context) ([]lease.Account, error) {
		return []lease.Account{{ID: account, Ceiling: odd}}, nil
	}))
	got := gathered(t, reg)
	rate := got[series{"mediated_mailbox_ratelimit_account_rate", account, ""}]
	floor := got[series{"mediated_mailbox_ratelimit_account_floor", account, ""}]
	if float64(float32(0.3)) == 0.3 {
		t.Fatal("0.3 is exact as a four-byte float, so the case shows nothing")
	}
	if !(rate <= floor) {
		t.Errorf("a rate held at the floor reads %v against a floor of %v, so the collapse rule could never fire", rate, floor)
	}
}

// A failed listing fails the scrape, so the series go absent and the absence rule fires, rather than
// a partial scrape reading as healthy (ADR-0077).
func TestTheCollectorFailsTheScrapeWhenItCannotRead(t *testing.T) {
	c := lease.NewCollector(spenders(t), func(context.Context) ([]lease.Account, error) {
		return nil, errors.New("no accounts")
	})
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	if _, err := reg.Gather(); err == nil {
		t.Error("the scrape succeeded though the collector could not list the accounts")
	}
}

// Every series the alerting rules read, apart from the two the provider adapters emit and the
// reload-failure series the policy loader emits, is one the collector or a Limiter emits under that
// exact name, so a rule never watches a name nothing emits (ADR-0076). The policy loader's own tests
// hold its series to the same check.
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	reg := prometheus.NewRegistry()
	metrics, err := lease.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lease.New(spenders(t), ceiling, metrics).Acquire(t.Context(), account, core.Batch, 1); err != nil {
		t.Fatal(err)
	}
	reg.MustRegister(lease.NewCollector(spenders(t), func(context.Context) ([]lease.Account, error) {
		return []lease.Account{{ID: account, Ceiling: ceiling}}, nil
	}))
	emitted := map[string]bool{requestCostName: true, hardCapName: true, reloadFailedName: true}
	for s := range gathered(t, reg) {
		emitted[s.name] = true
	}
	src, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	read := slices.Compact(slices.Sorted(slices.Values(regexp.MustCompile(`mediated_mailbox_[a-z_]+`).FindAllString(string(src), -1))))
	if len(read) < 5 {
		t.Fatalf("the rules read only %v", read)
	}
	for _, name := range read {
		if !emitted[name] {
			t.Errorf("the rules read %s, which nothing emits", name)
		}
	}
}
