package lease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
)

// leaseMillis is how long a lease lives and how long a grant stays in the one-second window.
const leaseMillis = 1000

// The bounds on how long Acquire waits before it asks again. A worker still waiting asks again at
// least once a lease period, so an abandoned ask stops counting as demand (ADR-0025). The longest
// wait is half a lease period, which leaves the rest of the period for the transaction that asks.
const (
	shortestWait = 20 * time.Millisecond
	longestWait  = leaseMillis / 2 * time.Millisecond
)

// latestStoredMillis is the latest instant written to the rate state, about the year 287,000.
// PostgreSQL's timestamps end in the year 294,276, and a saturated instant such as a backoff with no
// end is held here rather than failing the write.
const latestStoredMillis = 9_000_000_000_000_000

// ErrRefused is returned for a request that can never be granted, one costing more than one second's
// worth at the hard cap, naming no class, or costing nothing (ADR-0024). It is returned at once, never
// after waiting, so a caller keeps each call within that size.
var ErrRefused = errors.New("the request can never be granted")

// Limiter issues one provider's leases and records the outcomes of the calls they pay for, for every
// account of that provider. Every process that spends holds one, and they coordinate through each
// account's rate state and grants in the database (ADR-0025).
//
// Every method runs one transaction through db/tx, which sets the account. Inside it the account's
// rate state is locked, the database's clock is read in the next statement, the stored state is read,
// ratelimit/core decides, and the decision is written before the transaction commits and releases the
// lock. The transaction relies on read committed's snapshot per statement, PostgreSQL's default. The
// lock is taken after the transaction's first statements, so only a snapshot taken by each later
// statement sees the writes of the worker that held the lock before. Under repeatable read every
// contended ask would fail to serialize.
//
// The ceiling is the provider's declared budget per second, from which every limit is recomputed on
// every call. The target and hard cap stored in the row are written only for display and never read
// back.
type Limiter struct {
	db      tx.Beginner
	ceiling float64
	metrics *Metrics
	draw    func() float64
}

// New returns a Limiter over db for a provider whose declared ceiling is ceiling, recording what it
// does in metrics.
func New(db tx.Beginner, ceiling float64, metrics *Metrics) *Limiter {
	return &Limiter{db: db, ceiling: ceiling, metrics: metrics, draw: rand.Float64}
}

// Acquire returns a lease of at least cost tokens for class on the account, waiting until the bucket
// can grant it or ctx ends. The lease expires one second after it was issued, by the database's clock,
// and the call it pays for is made within that second.
//
// Each ask is stored whether or not it is granted, and a waiting worker asks again at least every half
// second. A request that can never be granted returns ErrRefused at once.
func (l *Limiter) Acquire(ctx context.Context, account string, class core.Class, cost float64) (core.Lease, error) {
	req := core.Request{Class: class, Tokens: roundUp32(cost)}
	for {
		d, wait, err := l.issue(ctx, account, req)
		if err != nil {
			// An ask cut off by ctx fails however the driver reports it, so a caller can tell a
			// cancelled wait from a failed one.
			return core.Lease{}, errors.Join(ctx.Err(), err)
		}
		switch d.Outcome {
		case core.Granted:
			return d.Lease, nil
		case core.Refused:
			return core.Lease{}, ErrRefused
		case core.Waiting:
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return core.Lease{}, ctx.Err()
		case <-timer.C:
		}
	}
}

// issue decides one request in one transaction and returns the decision and, for a request left
// waiting, how long to wait before asking again.
func (l *Limiter) issue(ctx context.Context, account string, req core.Request) (core.Decision, time.Duration, error) {
	var (
		d    core.Decision
		wait time.Duration
		rate float64
	)
	err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
		q := ratestate.New(t)
		now, err := l.lock(ctx, q, account)
		if err != nil {
			return err
		}
		row, err := l.read(ctx, q, account)
		if err != nil {
			return err
		}
		grants, err := q.Grants(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the grants: %w", err)
		}
		st, is := stateOf(row), issuanceOf(row)
		recent, grantLeases := recentOf(grants)
		is.Recent = recent
		next, decided := core.Issue(l.ceiling, st, is, grantLeases, req, now)
		d, rate = decided, float64(row.CurrentRate)
		if d.Outcome == core.Refused {
			return nil
		}
		wait = l.waitFor(st, next, req, now)
		return l.storeIssuance(ctx, q, account, st, next, grantLeases, d, now)
	})
	if err != nil {
		return core.Decision{}, 0, err
	}
	l.metrics.observeRate(account, rate)
	if d.Outcome == core.Granted {
		l.metrics.observeGrant(account, d.Lease)
	}
	return d, wait, nil
}

// storeIssuance writes what issuance keeps after one request. The grants more than a second old are
// deleted, a grant stamped ahead of the instant issuance reached is moved back to it as the rules take
// it, and a granted lease gets its row.
func (l *Limiter) storeIssuance(ctx context.Context, q *ratestate.Queries, account string, st core.State, next core.Issuance, held []core.Lease, d core.Decision, now int64) error {
	limits := core.LimitsFor(l.ceiling)
	if err := q.DeleteGrantsBefore(ctx, ratestate.DeleteGrantsBeforeParams{AccountID: account, CutoffMs: stored(now - leaseMillis)}); err != nil {
		return fmt.Errorf("deleting old grants: %w", err)
	}
	if err := q.ClampGrants(ctx, ratestate.ClampGrantsParams{AccountID: account, NowMs: stored(next.At)}); err != nil {
		return fmt.Errorf("moving grants stamped ahead: %w", err)
	}
	var grantedAt int64
	if d.Outcome == core.Granted {
		grantedAt = stored(next.At)
		held = append(held, d.Lease)
		err := q.InsertGrant(ctx, ratestate.InsertGrantParams{
			AccountID: account,
			Class:     className(d.Lease.Class),
			Tokens:    float32(d.Lease.Tokens),
			IssuedMs:  grantedAt,
		})
		if err != nil {
			return fmt.Errorf("storing the grant: %w", err)
		}
	}
	classes, err := classesShown(st, limits, held, next.At)
	if err != nil {
		return err
	}
	return q.UpdateIssuance(ctx, ratestate.UpdateIssuanceParams{
		AccountID:          account,
		TargetRate:         float32(limits.Target),
		HardCap:            float32(limits.HardCap),
		BucketLevel:        roundDown32(next.Level),
		BucketFilledMs:     stored(next.At),
		InteractiveAskedMs: stored(next.Asked[core.Interactive]),
		SyncAskedMs:        stored(next.Asked[core.Sync]),
		BatchAskedMs:       stored(next.Asked[core.Batch]),
		GrantedMs:          grantedAt,
		Classes:            classes,
		NowMs:              stored(now),
	})
}

// waitFor returns how long a request left waiting waits before it asks again. It waits out a backoff,
// or the time the bucket needs to refill to the request at the rate it refills at, and never longer
// than half a lease period. A request held back by a reservation or the one-second window waits the
// shortest time.
func (l *Limiter) waitFor(st core.State, next core.Issuance, req core.Request, now int64) time.Duration {
	wait := shortestWait
	switch rate := min(st.Rate, core.LimitsFor(l.ceiling).Target); {
	case st.BackoffUntil > now:
		wait = time.Duration(min(st.BackoffUntil-now, leaseMillis)) * time.Millisecond
	case rate > 0 && req.Tokens > next.Level:
		wait = time.Duration(min((req.Tokens-next.Level)/rate, 1) * float64(time.Second))
	}
	return min(max(wait, shortestWait), longestWait)
}

// Succeeded records that a call of the given cost succeeded after latency, measured by the caller on
// its own clock. The rate grows, a run of throttles ends, and the latency joins the account's current
// one-minute window. When that window has closed, its median is judged against the baseline of the
// windows before it and then joins them (ADR-0024).
func (l *Limiter) Succeeded(ctx context.Context, account string, cost mail.OpCost, latency time.Duration) error {
	return l.control(ctx, account, func(st core.State, lat latencyRecord, now int64) (core.State, latencyRecord, int64) {
		st = core.Succeeded(st, l.ceiling, cost)
		next, median, closed := lat.window.Add(now, latency.Milliseconds())
		lat.window = next
		if closed {
			st = core.LatencyMeasured(st, l.ceiling, median, core.Baseline(lat.medians))
			lat.medians = core.RememberMedian(lat.medians, median)
		}
		return st, lat, 0
	})
}

// Throttled records that the provider throttled a call. The rate halves and nothing is issued until
// the signal's retry-after, or a full-jitter backoff without one, has passed.
func (l *Limiter) Throttled(ctx context.Context, account string, signal mail.ThrottleSignal) error {
	draw := l.draw()
	err := l.control(ctx, account, func(st core.State, lat latencyRecord, now int64) (core.State, latencyRecord, int64) {
		return core.Throttled(st, l.ceiling, signal, now, draw), lat, now
	})
	if err == nil {
		l.metrics.observeThrottle(account, signal.Scope)
	}
	return err
}

// ServerErrored records that the provider failed a call with a server error. The rate falls to 80%.
func (l *Limiter) ServerErrored(ctx context.Context, account string) error {
	return l.control(ctx, account, func(st core.State, lat latencyRecord, _ int64) (core.State, latencyRecord, int64) {
		return core.ServerErrored(st, l.ceiling), lat, 0
	})
}

// latencyRecord is the account's latency record, the current window and the last ten window medians.
type latencyRecord struct {
	window  core.LatencyWindow
	medians []float64
}

// control runs one controller rule over the account's rate state in one transaction. decide returns
// the new state, the new latency record and the instant of a throttle, zero when there was none.
func (l *Limiter) control(ctx context.Context, account string, decide func(core.State, latencyRecord, int64) (core.State, latencyRecord, int64)) error {
	var rate float64
	err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
		q := ratestate.New(t)
		now, err := l.lock(ctx, q, account)
		if err != nil {
			return err
		}
		row, err := l.read(ctx, q, account)
		if err != nil {
			return err
		}
		st, lat, throttledAt := decide(stateOf(row), latencyOf(row), now)
		rate = st.Rate
		limits := core.LimitsFor(l.ceiling)
		return q.UpdateController(ctx, ratestate.UpdateControllerParams{
			AccountID:            account,
			CurrentRate:          float32(st.Rate),
			TargetRate:           float32(limits.Target),
			HardCap:              float32(limits.HardCap),
			BackoffUntilMs:       stored(st.BackoffUntil),
			Throttles:            throttlesStored(st.Throttles),
			ThrottledMs:          stored(throttledAt),
			LatencyWindowStartMs: stored(lat.window.Start),
			LatencySamples:       samplesStored(lat.window.Samples),
			LatencyMedians:       mediansStored(lat.medians),
			BaselineMs:           float32(core.Baseline(lat.medians)),
			NowMs:                stored(now),
		})
	})
	if err == nil {
		l.metrics.observeRate(account, rate)
	}
	return err
}

// lock takes the account's lock and then reads the database's clock in a statement of its own, so a
// worker that waited on the lock is stamped with the time it got it (ADR-0025).
func (l *Limiter) lock(ctx context.Context, q *ratestate.Queries, account string) (int64, error) {
	if err := q.LockRateState(ctx, account); err != nil {
		return 0, fmt.Errorf("locking the rate state: %w", err)
	}
	now, err := q.ClockMillis(ctx)
	if err != nil {
		return 0, fmt.Errorf("reading the database's clock: %w", err)
	}
	return now, nil
}

// read returns the account's rate state, giving the account one at the target the first time it
// spends.
func (l *Limiter) read(ctx context.Context, q *ratestate.Queries, account string) (ratestate.RateStateRow, error) {
	limits := core.LimitsFor(l.ceiling)
	err := q.InsertRateState(ctx, ratestate.InsertRateStateParams{
		AccountID:   account,
		CurrentRate: float32(limits.Target),
		TargetRate:  float32(limits.Target),
		HardCap:     float32(limits.HardCap),
	})
	if err != nil {
		return ratestate.RateStateRow{}, fmt.Errorf("giving the account its rate state: %w", err)
	}
	row, err := q.RateState(ctx, account)
	if err != nil {
		return ratestate.RateStateRow{}, fmt.Errorf("reading the rate state: %w", err)
	}
	return row, nil
}

func stateOf(row ratestate.RateStateRow) core.State {
	return core.State{Rate: float64(row.CurrentRate), BackoffUntil: row.BackoffUntilMs, Throttles: int(row.Throttles)}
}

func issuanceOf(row ratestate.RateStateRow) core.Issuance {
	var asked [core.Batch + 1]int64
	asked[core.Interactive], asked[core.Sync], asked[core.Batch] = row.InteractiveAskedMs, row.SyncAskedMs, row.BatchAskedMs
	return core.Issuance{Level: float64(row.BucketLevel), At: row.BucketFilledMs, Asked: asked}
}

func latencyOf(row ratestate.RateStateRow) latencyRecord {
	lat := latencyRecord{window: core.LatencyWindow{Start: row.LatencyWindowStartMs}}
	for _, s := range row.LatencySamples {
		lat.window.Samples = append(lat.window.Samples, int64(s))
	}
	for _, m := range row.LatencyMedians {
		lat.medians = append(lat.medians, float64(m))
	}
	return lat
}

// recentOf returns the stored grants as the one-second window counts them and as the leases they
// are. Every row is passed on, and the rules leave out the grants older than a second.
func recentOf(grants []ratestate.GrantsRow) ([]core.Issued, []core.Lease) {
	recent := make([]core.Issued, 0, len(grants))
	leases := make([]core.Lease, 0, len(grants))
	for _, g := range grants {
		recent = append(recent, core.Issued{At: g.IssuedMs, Tokens: float64(g.Tokens)})
		leases = append(leases, core.Lease{Class: classNamed(g.Class), Tokens: float64(g.Tokens), Expires: g.IssuedMs + leaseMillis})
	}
	return recent, leases
}

// classesShown returns the per-class display the row's classes column holds, each class's reserved
// share at the current rate and the tokens its live leases hold (ADR-0025).
func classesShown(st core.State, limits core.Limits, held []core.Lease, now int64) ([]byte, error) {
	type shown struct {
		Reserved float64 `json:"reserved"`
		Used     float64 `json:"used"`
	}
	rate := min(max(st.Rate, limits.Floor), limits.Target)
	out := map[string]shown{}
	for c := core.Interactive; c <= core.Batch; c++ {
		s := shown{Reserved: core.Share(c, rate, limits.Target)}
		for _, h := range core.Live(held, now) {
			if h.Class == c && h.Tokens > 0 {
				s.Used += h.Tokens
			}
		}
		out[className(c)] = s
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encoding the classes shown: %w", err)
	}
	return b, nil
}

// The names a grant's class is stored under.
var classNames = [core.Batch + 1]string{core.Interactive: "interactive", core.Sync: "sync", core.Batch: "batch"}

func className(c core.Class) string {
	if c < core.Interactive || c > core.Batch {
		return ""
	}
	return classNames[c]
}

// classNamed returns the class a stored name names, or no class. A grant of no class still counts in
// the one-second window, and against no class's share.
func classNamed(name string) core.Class {
	for c := core.Interactive; c <= core.Batch; c++ {
		if classNames[c] == name {
			return c
		}
	}
	return 0
}

// stored returns an instant as the rate state stores it, held between the Unix epoch and
// latestStoredMillis. Zero stores no instant.
func stored(ms int64) int64 {
	return min(max(ms, 0), latestStoredMillis)
}

// roundUp32 returns the smallest four-byte float not below x, the precision a grant's tokens are
// stored at. A request is rounded up before it is decided, so the grant stored is exactly the grant
// decided and never less than the call's cost. A value that is not finite is left for the rules to
// refuse.
func roundUp32(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || x > math.MaxFloat32 {
		return x
	}
	f := float32(x)
	if float64(f) < x {
		f = math.Nextafter32(f, float32(math.Inf(1)))
	}
	return float64(f)
}

// roundDown32 returns the largest four-byte float not above x, so the bucket's stored level never
// holds more than the rules left in it.
func roundDown32(x float64) float32 {
	if math.IsNaN(x) {
		return 0
	}
	f := float32(x)
	if float64(f) > x {
		f = math.Nextafter32(f, float32(math.Inf(-1)))
	}
	return f
}

// samplesStored returns latency samples as the rate state stores them, in whole milliseconds held
// within the column's range.
func samplesStored(samples []int64) []int32 {
	out := make([]int32, 0, len(samples))
	for _, s := range samples {
		out = append(out, int32(min(max(s, 0), math.MaxInt32)))
	}
	return out
}

// throttlesStored returns a count of throttles held within the column's range.
func throttlesStored(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

func mediansStored(medians []float64) []float32 {
	out := make([]float32, 0, len(medians))
	for _, m := range medians {
		out = append(out, float32(m))
	}
	return out
}
