//go:build integration

package lease_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// realCeiling is the provider's real ceiling in the convergence test, below the 50 units a second the
// declared ceiling's target allows, as a provider that throttles below its documented number does.
const realCeiling = 30

// rateOf reads the account's stored rate.
func rateOf(t *testing.T, conn *pgx.Conn, account string) float64 {
	t.Helper()
	var rate float64
	if err := conn.QueryRow(t.Context(), "SELECT current_rate FROM rate_state WHERE account_id = $1", account).Scan(&rate); err != nil {
		t.Fatal(err)
	}
	return rate
}

// The controller runs against the provider fake, throttling any second's calls above a real ceiling
// the controller does not know. Two workers lease for every call, make it, and report its outcome.
// The rate is cut when the provider throttles, climbs again after it, and settles below the real
// ceiling, and no second of grants holds more than the hard cap (ADR-0024, ADR-0025).
func TestTheControllerConvergesBelowARealCeilingAndRecovers(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	mailbox, err := fake.New(fake.Config{Account: account, BudgetPerSecond: ceiling})
	if err != nil {
		t.Fatal(err)
	}
	signal := mail.ThrottleSignal{RetryAfterMillis: 200, HasRetryAfter: true, Scope: mail.ScopePerUser}
	port := fake.Throttle(mailbox, fake.Ceiling(realCeiling, signal), time.Now)
	profile := port.RateProfile()
	l := lease.New(spenders(t), profile.BudgetPerSecond(), nil)

	ids := []string{"m1", "m2", "m3", "m4", "m5"}
	cost := profile.Cost(mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: len(ids)})
	var g ledger
	var throttles atomic.Int64
	work := func(ctx context.Context) {
		for ctx.Err() == nil {
			got, err := l.Acquire(ctx, account, core.Batch, cost.Weight)
			if err != nil {
				continue
			}
			g.add(got)
			began := time.Now()
			_, err = port.GetMessageMetadata(ctx, ids)
			if s, ok := profile.ParseThrottle(err); ok {
				throttles.Add(1)
				err = l.Throttled(ctx, account, s)
			} else if err == nil {
				err = l.Succeeded(ctx, account, cost, time.Since(began))
			}
			// A call or report the deadline cut off is not the test's subject.
			if err != nil && ctx.Err() == nil {
				t.Errorf("a call or its report failed: %v", err)
			}
		}
	}

	const run = 20 * time.Second
	var rates []float64
	var wg sync.WaitGroup
	wg.Go(func() { until(run, 2, work) })
	for elapsed := time.Duration(0); elapsed < run; elapsed += 250 * time.Millisecond {
		time.Sleep(250 * time.Millisecond)
		rates = append(rates, rateOf(t, conn, account))
	}
	wg.Wait()

	if throttles.Load() == 0 {
		t.Fatal("the provider never throttled, so the controller never met its real ceiling")
	}
	lowest := 0
	for i := range rates[:len(rates)/2] {
		if rates[i] < rates[lowest] {
			lowest = i
		}
	}
	highestAfter := rates[lowest]
	for _, r := range rates[lowest:] {
		highestAfter = max(highestAfter, r)
	}
	if highestAfter < rates[lowest]+3 {
		t.Errorf("after falling to %v the rate never climbed above %v, so it did not recover", rates[lowest], highestAfter)
	}
	settled := rates[len(rates)/2:]
	mean := 0.0
	for _, r := range settled {
		mean += r
	}
	mean /= float64(len(settled))
	if mean >= realCeiling {
		t.Errorf("the rate averaged %v over the last ten seconds, not below the real ceiling of %d", mean, realCeiling)
	}
	if most := busiestSecond(g.all()); most > hardCap {
		t.Errorf("%v tokens were issued inside one second, past the hard cap of %d", most, hardCap)
	}
}
