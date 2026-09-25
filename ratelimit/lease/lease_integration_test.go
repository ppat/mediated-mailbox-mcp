//go:build integration

package lease_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// spend asks for leases of cost for class until ctx ends, recording each grant.
func spend(ctx context.Context, l *lease.Limiter, account string, class core.Class, cost float64, g *ledger) {
	for ctx.Err() == nil {
		got, err := l.Acquire(ctx, account, class, cost)
		if err == nil {
			g.add(got)
		}
	}
}

// A request costing more than one second's worth at the hard cap is refused at once rather than left
// waiting, and one of exactly that size is granted, so the refusal is the size's doing (ADR-0024,
// ADR-0023).
func TestARequestPastTheHardCapIsRefusedAtOnce(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	l := lease.New(spenders(t), ceiling, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if got, err := l.Acquire(ctx, account, core.Interactive, hardCap+0.5); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("Acquire of %v tokens = %+v, %v, want ErrRefused", hardCap+0.5, got, err)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Errorf("the refusal came after %v, want it at once", waited)
	}
	got, err := l.Acquire(ctx, account, core.Interactive, hardCap)
	if err != nil || got.Tokens != hardCap {
		t.Errorf("Acquire of %d tokens = %+v, %v, want them granted", hardCap, got, err)
	}
}

// A hard cap, target, rate and bucket level far above what the declared ceiling gives, written into
// the rate state, change nothing. The issuer recomputes the cap from the ceiling and never reads the
// stored one, and it holds the stored level to the bucket's capacity, so no second holds more than
// the hard cap's worth (ADR-0024, ADR-0016).
func TestAStoredCapAboveTheCeilingDoesNotRaiseIssuance(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, bucket_level, bucket_filled_at)
		VALUES ($1, 1e6, 1e6, 1e6, 1e6, now() - interval '1 hour')`, account)
	l := lease.New(spenders(t), ceiling, nil)

	var g ledger
	until(2500*time.Millisecond, 4, func(ctx context.Context) { spend(ctx, l, account, core.Batch, 10, &g) })

	leases := g.all()
	total := 0.0
	for _, got := range leases {
		total += got.Tokens
	}
	if total <= 2*hardCap {
		t.Fatalf("only %v tokens were issued in 2.5 seconds, too few for the test to show the cap", total)
	}
	if most := busiestSecond(leases); most > hardCap {
		t.Errorf("%v tokens were issued inside one second, past the hard cap of %d", most, hardCap)
	}
}

// A worker that waited on the account's lock is stamped with the clock read after it got the lock,
// not before. So a grant the waiting worker receives counts in the one-second window from when it was
// really issued, and a later request cannot take a second hard cap's worth inside that real second
// (ADR-0025).
func TestAWorkerThatWaitedOnTheLockIsStampedAfterIt(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	pool := spenders(t)
	l := lease.New(pool, ceiling, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	held, release := make(chan struct{}), make(chan struct{})
	var releasedAt int64
	holder := make(chan error, 1)
	go func() {
		holder <- tx.Run(ctx, pool, account, func(t pgx.Tx) error {
			q := ratestate.New(t)
			if err := q.LockRateState(ctx, account); err != nil {
				return err
			}
			close(held)
			<-release
			var err error
			releasedAt, err = q.ClockMillis(ctx)
			return err
		})
	}()
	<-held

	type result struct {
		lease  core.Lease
		err    error
		waited time.Duration
	}
	waiter := make(chan result, 1)
	go func() {
		start := time.Now()
		got, err := l.Acquire(ctx, account, core.Interactive, 20)
		waiter <- result{got, err, time.Since(start)}
	}()
	time.Sleep(700 * time.Millisecond)
	close(release)
	if err := <-holder; err != nil {
		t.Fatalf("holding the lock: %v", err)
	}
	first := <-waiter
	if first.err != nil {
		t.Fatalf("the waiting worker's Acquire: %v", first.err)
	}
	if first.waited < 500*time.Millisecond {
		t.Fatalf("the worker was granted after %v, so it never waited on the lock", first.waited)
	}
	if at := issued(first.lease); at < releasedAt {
		t.Errorf("the waiting worker's grant is stamped %d ms before the lock was released", releasedAt-at)
	}

	second, err := l.Acquire(ctx, account, core.Interactive, hardCap)
	if err != nil {
		t.Fatal(err)
	}
	if gap := issued(second) - releasedAt; gap < 1000 {
		t.Errorf("a hard cap's worth was granted %d ms after the waiting worker's grant was really issued, inside one second", gap)
	}
}

// askedAt reads the instant a class last asked, as stored, in Unix milliseconds, or zero.
func askedAt(t *testing.T, conn *pgx.Conn, account string, index int) int64 {
	t.Helper()
	var at int64
	err := conn.QueryRow(t.Context(),
		"SELECT coalesce(floor(extract(EPOCH FROM class_asked_at[$2]) * 1000), 0)::bigint FROM rate_state WHERE account_id = $1",
		account, index).Scan(&at)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	return at
}

// A request the bucket cannot fill records its class's ask instant on the first ask, again within
// every lease period while the worker waits, and never again once the worker stops waiting, so the
// collapse rules read demand that is real and ends when a worker gives up (ADR-0025, ADR-0077).
func TestAnAskIsRecordedAndRenewedUntilTheWorkerStops(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	// A backoff lasting an hour, so the request waits throughout.
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, backoff_until)
		VALUES ($1, 50, 50, 80, now() + interval '1 hour')`, account)
	l := lease.New(spenders(t), ceiling, nil)

	ctx, stop := context.WithCancel(t.Context())
	done := make(chan error, 1)
	start := clock(t, conn)
	go func() {
		_, err := l.Acquire(ctx, account, core.Sync, 10)
		done <- err
	}()

	var seen []int64
	for range 60 {
		if at := askedAt(t, conn, account, 2); at != 0 && (len(seen) == 0 || at != seen[len(seen)-1]) {
			seen = append(seen, at)
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("the waiting Acquire returned %v, want it cancelled", err)
	}
	if len(seen) < 3 {
		t.Fatalf("the sync class's ask instant took %d values in three seconds of waiting, want one per lease period at least", len(seen))
	}
	if first := seen[0] - start; first > 500 {
		t.Errorf("the first ask was recorded %d ms after the worker asked", first)
	}
	for i := 1; i < len(seen); i++ {
		if gap := seen[i] - seen[i-1]; gap > 1000 {
			t.Errorf("the ask instant went %d ms without renewal while the worker waited, longer than a lease period", gap)
		}
	}
	// A transaction the cancellation cut off can still commit on the server after Acquire returns, so
	// the instant is read once that has settled.
	time.Sleep(time.Second)
	last := askedAt(t, conn, account, 2)
	time.Sleep(1500 * time.Millisecond)
	if after := askedAt(t, conn, account, 2); after != last {
		t.Errorf("the ask instant moved %d ms after the worker stopped asking", after-last)
	}
}

// A crashed worker's lease counts against its class's share until it expires one second after it was
// issued, and then stops, which is how its tokens return to the pool. The rate is 10, so
// interactive's share is 10, and the bucket starts at 30, so the bucket rather than the one-second
// window decides what is granted. An interactive worker takes the class's whole share and crashes.
// Batch workers asking for 2 at a time then drain the bucket as it refills, and another interactive
// request for 10 waits. Once the crashed lease expires, interactive's share is reserved against batch
// again, the bucket keeps 10 for it, and that request is granted. A lease that never expired would
// leave interactive starved behind batch (ADR-0025).
func TestACrashedWorkersLeaseStopsCountingAtItsExpiry(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, bucket_level, bucket_filled_at)
		VALUES ($1, 10, 50, 80, 30, clock_timestamp())`, account)
	l := lease.New(spenders(t), ceiling, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	crashed, err := l.Acquire(ctx, account, core.Interactive, 10)
	if err != nil {
		t.Fatal(err)
	}
	var g ledger
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { spend(ctx, l, account, core.Batch, 2, &g) })
	}
	// Batch drains what the crashed lease left, so the next interactive request finds the bucket
	// empty while the crashed lease still counts.
	for tokensIssuedBetween(g.all(), 0, crashed.Expires) < 16 {
		if ctx.Err() != nil {
			t.Fatal("batch never drained the bucket")
		}
		time.Sleep(5 * time.Millisecond)
	}
	next, err := l.Acquire(ctx, account, core.Interactive, 10)
	cancel()
	wg.Wait()
	if err != nil {
		t.Fatalf("interactive was not granted its share while batch drained the bucket: %v", err)
	}
	if issued(next) < crashed.Expires {
		t.Errorf("interactive was granted %d ms before the crashed lease expired, while batch drained the bucket", crashed.Expires-issued(next))
	}
}

// A live lease counts against its class's share. Interactive holds its whole share, so it reserves
// nothing more against batch, and batch is granted everything else in the bucket at once. A lease
// that did not count, or counted under another class, would keep interactive's share reserved and
// batch waiting. The rate sits at the floor so the bucket refills too slowly to hide the difference
// (ADR-0025).
func TestALiveLeaseCountsAgainstItsClassShare(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	must(t, conn, "INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap) VALUES ($1, 5, 50, 80)", account)
	l := lease.New(spenders(t), ceiling, nil)

	// At the floor interactive's share is the whole rate, 5.
	if _, err := l.Acquire(t.Context(), account, core.Interactive, 15); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	if _, err := l.Acquire(ctx, account, core.Batch, hardCap-15); err != nil {
		t.Errorf("batch was not granted the rest of the bucket beside interactive's live lease: %v", err)
	}
}

// Batch holds leases up to the whole budget, then two throttles cut the rate to a quarter of the
// target. Interactive, asking within its share, is granted each request within one lease period,
// and batch absorbs the cut, getting only what interactive leaves (O1, ADR-0025).
func TestACutComesOutOfBatchWhileInteractiveKeepsItsShare(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn)
	l := lease.New(spenders(t), ceiling, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	start := clock(t, conn)

	var batch ledger
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { spend(ctx, l, account, core.Batch, 5, &batch) })
	}
	time.Sleep(time.Second)
	signal := mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true, Scope: mail.ScopePerUser}
	for range 2 {
		if err := l.Throttled(ctx, account, signal); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// The rate is now 12.5, so interactive's share is 12.5, and it asks for 10 a second.
	var slowest time.Duration
	var asked int
	for ctx.Err() == nil {
		began := time.Now()
		if _, err := l.Acquire(ctx, account, core.Interactive, 5); err != nil {
			break
		}
		asked++
		slowest = max(slowest, time.Since(began))
		time.Sleep(500*time.Millisecond - time.Since(began))
	}
	wg.Wait()
	if asked < 5 {
		t.Fatalf("interactive was granted %d requests in its 3.8 seconds, want one every half second", asked)
	}
	if slowest >= time.Second {
		t.Errorf("an interactive request within its share waited %v, a lease period or more", slowest)
	}
	// Two seconds settled after the cut, the bucket refills 25 and interactive takes 20.
	if got := tokensIssuedBetween(batch.all(), start+3000, start+5000); got > 15 {
		t.Errorf("batch was granted %v tokens in two seconds after the cut, want what interactive leaves of 12.5 a second", got)
	}
}
