//go:build integration

package backfill

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// The user backfill connects as, the application user, so its grants apply as in production.
const role = postgres.ApplicationUser

// backfillPool returns a pool connecting as role.
func backfillPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// superuser returns a connection that holds every privilege, to write what the UI's setups
// would write and to break what the tests break.
func superuser(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// reset empties the tables backfill reads and writes. Every test of the package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state", "account_state", "accounts", "oauth_clients",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// gmailProvider is the provider the tests' accounts name.
const gmailProvider = "gmail"

// account writes an account, and a state row holding credential unless stateless.
func account(t *testing.T, conn *pgx.Conn, id, provider string, credential []byte, stateless bool) {
	t.Helper()
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, $2)", id, provider)
	if !stateless {
		must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", id, credential)
	}
}

// logBuffer collects the records a run logs.
type logBuffer struct{ bytes.Buffer }

func (b *logBuffer) logger() *slog.Logger { return slog.New(slog.NewJSONHandler(b, nil)) }

// counted is the units of work of a pass a test runs with no connection, counting each end and
// failing the end it is told to with failure.
type counted struct {
	opened, ended int
	// failAt is the end, counted from one, that returns failure, zero for none.
	failAt  int
	failure error
}

func (c *counted) open() error { c.opened++; return nil }

func (c *counted) end(context.Context) error {
	c.ended++
	if c.ended == c.failAt {
		return c.failure
	}
	return nil
}

// idleUnit is the units of work of a pass whose ends a test does not read.
func idleUnit(*testing.T, *pgx.Conn, string) units { return &counted{} }

var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// fixed returns the policy function that always returns c.
func fixed(c classify.Policy) func() classify.Policy { return func() classify.Policy { return c } }

// three is the call size of the tests' passes, how many identifiers one call fetching stale subjects
// again names.
func three() int { return 3 }

// firstPassDeps returns the dependencies of a pass over the account through pool, enumerating a
// provider fake holding messages in pages of two, one of them from a sender without a domain.
func firstPassDeps(t *testing.T, pool *pgxpool.Pool, account string) pass1.Deps {
	t.Helper()
	var messages []fake.Message
	for i, from := range []string{"a@news.example", "b@news.example", "no-domain", "c@shop.example", "d@news.example"} {
		messages = append(messages, fake.Message{Metadata: mail.MessageMetadata{
			ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: from},
			Subject: marker.Field("subject"), Date: mail.UnixMilli(1_700_000_000_000 + i),
		}})
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	s, err := scan.New(config.DefaultScanner(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	none, err := policy.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	return pass1.Deps{
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll, Metadata: f.GetMessageMetadata, PerCall: three,
		Policy: fixed(classify.Prepare(none.For(account), lookups)), Scanner: s,
		RunID: func() string { n++; return fmt.Sprintf("run%d", n) },
		Now:   time.Now,
	}
}

// Every page of the first pass is a unit of work. A unit of work is begun before each page and ended
// after it, writing a rotated credential back, so a rotation reaches the database a page after it
// happens rather than at the end of the run (ADR-0082). The messages whose sender the classifier
// could not classify are counted on the account's series as each page is made durable (O2), and so are
// the messages each page added to the index, each once (ADR-0125).
func TestEachPageIsAUnitOfWork(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	registry := prometheus.NewRegistry()
	messages, err := series.NewMessages(registry)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := pass1.NewMetrics(registry, messages)
	if err != nil {
		t.Fatal(err)
	}
	u := &counted{}

	if err := passAccount(t.Context(), firstPassDeps(t, pool, "personal"), "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	if u.opened != 3 || u.ended != 3 {
		t.Errorf("the units of work began %d and ended %d times, want once each for each of the three pages", u.opened, u.ended)
	}
	if got := counterValue(t, registry, series.UnclassifiedName, "personal"); got != 1 {
		t.Errorf("the unclassified senders series reads %v, want 1", got)
	}
	var stored float64
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM messages WHERE account_id = 'personal'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if got := stageValue(t, registry, "personal", series.StageIndexed); got != stored || stored == 0 {
		t.Errorf("the messages series counts %v indexed, want the %v messages the index holds", got, stored)
	}
}

// stageValue returns the account's count of messages made durable at stage on registry, or zero when
// none was counted.
func stageValue(t *testing.T, registry prometheus.Gatherer, account, stage string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != series.MessagesName {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["account"] == account && labels["stage"] == stage {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// stepCount returns how many times the body processing series on registry observed step.
func stepCount(t *testing.T, registry prometheus.Gatherer, step string) uint64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != scanner.ProcessingName {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "step" && l.GetValue() == step {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	return 0
}

// The end of a unit of work that fails is returned once the pass ends, joined with every other, and
// the pass still ends, since the connections keep the rotated credential in memory and a later
// write-back may land it (ADR-0082).
func TestAFailedHandOverFailsThePassAtItsEnd(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("account personal: recording the last authentication attempt failed")
	u := &counted{failAt: 1, failure: failed}

	err = passAccount(t.Context(), firstPassDeps(t, backfillPool(t), "personal"), "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler))

	if !errors.Is(err, failed) {
		t.Errorf("passAccount returned %v, want the failed end of a unit of work", err)
	}
	var ended bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass1_complete FROM account_state WHERE account_id = 'personal'").Scan(&ended); err != nil || !ended {
		t.Errorf("the pass did not end after a failed end of a unit of work (%v)", err)
	}
	if u.ended != 3 {
		t.Errorf("the units of work ended %d times, want three, the pass going on after the failed one", u.ended)
	}
}

// counterValue returns the value of the series name carries for account, failing when it has none.
func counterValue(t *testing.T, registry prometheus.Gatherer, name, account string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" && l.GetValue() == account {
					if m.GetCounter() != nil {
						return m.GetCounter().GetValue()
					}
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	t.Fatalf("no series %s for account %s", name, account)
	return 0
}

// accountsIn returns the account of every series of name reg holds.
func accountsIn(t *testing.T, reg *prometheus.Registry, name string) []string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" {
					out = append(out, l.GetValue())
				}
			}
		}
	}
	return out
}

// A page that fails is still a unit of work, so its unit is ended after it too, and a rotation made
// before the failure reaches the database (ADR-0082).
func TestAFailedPageIsStillHandedOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	deps := firstPassDeps(t, backfillPool(t), "personal")
	deps.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, mail.ErrAuthentication
	}
	u := &counted{}

	err = passAccount(t.Context(), deps, "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler))

	if !errors.Is(err, mail.ErrAuthentication) {
		t.Errorf("passAccount returned %v, want the refused credential", err)
	}
	if u.ended != 1 {
		t.Errorf("the units of work ended %d times after the failed page, want once", u.ended)
	}
}

// VERIFICATIONS' row for a job's latest success read from recorded state, backfill's part. Each
// account's job, as the plan's listing makes it, starts from its latest success as its runs record it, so a worker that restarts
// without the job succeeding leaves it ageing. An account with a pass not ended counts from its
// latest page made durable or run that succeeded, a first pass in a crash loop, with runs and no
// success, from its earliest run's start, and an account whose backfill has ended, or that has no
// run, from its ensure (ADR-0119).
func TestEachJobStartsFromItsRecordedLatestSuccess(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	for _, id := range []string{"progressed", "succeeded", "looping", "ended", "new"} {
		account(t, conn, id, gmailProvider, nil, false)
	}
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := func(account, id, pass, state string, started time.Time, finished *time.Time) {
		must(t, conn, "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at) VALUES ($1, $2, 'backfill', $3, $4, $5, $6)",
			account, id, pass, state, started, finished)
	}
	event := func(account, id, kind string, at time.Time) {
		must(t, conn, "INSERT INTO job_run_events (account_id, run_id, kind, at) VALUES ($1, $2, $3, $4)", account, id, kind, at)
	}
	later := func(d time.Duration) *time.Time { v := t0.Add(d); return &v }

	// A first pass that made pages durable and then stopped, and was resumed by a run that has made
	// none, so its latest success is its last page.
	run("progressed", "p1", "pass1", "failed", t0, later(time.Hour))
	event("progressed", "p1", "progress", t0.Add(30*time.Minute))
	event("progressed", "p1", "failure", t0.Add(time.Hour))
	run("progressed", "p2", "pass1", "running", t0.Add(2*time.Hour), nil)
	// A first pass that succeeded, with the second not yet ended, so its latest success is the end of
	// the run that succeeded, later than its last page.
	run("succeeded", "s1", "pass1", "succeeded", t0, later(3*time.Hour))
	event("succeeded", "s1", "progress", t0.Add(2*time.Hour))
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true WHERE account_id = 'succeeded'")
	// A first pass in a crash loop, each run stopped before a page was made durable.
	run("looping", "l1", "pass1", "failed", t0.Add(time.Hour), later(time.Hour+time.Minute))
	run("looping", "l2", "pass1", "running", t0.Add(2*time.Hour), nil)
	// An account whose backfill ended long ago.
	run("ended", "e1", "pass2", "succeeded", t0, later(time.Hour))
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'ended'")

	plan := (&Backfill{pool: backfillPool(t), logger: slog.New(slog.DiscardHandler), seeding: t.Context()}).plan()
	got := map[string]time.Time{}
	for _, id := range []string{"progressed", "succeeded", "looping", "ended", "new"} {
		job, err := plan.Job(id)
		if err != nil {
			t.Fatal(err)
		}
		got[id] = job.LastSuccess.UTC()
	}
	want := map[string]time.Time{
		"progressed": t0.Add(30 * time.Minute),
		"succeeded":  t0.Add(3 * time.Hour),
		"looping":    t0.Add(time.Hour),
		"ended":      {},
		"new":        {},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the latest success each job starts from (-want +got):\n%s", diff)
	}
}
