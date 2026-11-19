package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database/core"
)

// The bounds of a pool's record. A pool that holds no connection and has made none, and run no
// statement the database answered, for noConnectionBound since its first failed connection attempt
// that counted reports failed,
// which fails the health probe so the platform restarts the process, rebuilding the pool and reading
// its password file and its server's name again. The bound is long, since a database outage that
// outlasts it restarts every process that connects, and a process that restarts while its database
// cannot be reached fails its start's schema check and waits out the platform's growing restart delay.
// healthCheckPeriod is how often a pool that holds no connection attempts one on its own. A pool
// that is not ready soon holds none, so that attempt is the evidence a not-ready report clears on
// while the process takes no traffic, and it is the most attempts one pool makes of a database that
// does not answer. connectTimeout bounds one attempt, so an attempt the network never answers ends as
// a failed attempt rather than as its caller giving up, which is no evidence (ADR-0142).
const (
	noConnectionBound = 15 * time.Minute
	healthCheckPeriod = 10 * time.Second
	connectTimeout    = 10 * time.Second
)

// The reasons a pool's report gives. They carry no error's detail, which the record logs at debug.
const unreachable = "the database could not serve the latest statement or connection attempt"

var disconnected = fmt.Sprintf("no connection to the database has been made for more than %s", noConnectionBound)

// record is what one pool's statements and connection attempts have shown about its database, the
// evidence its readiness report is made from (ADR-0142). Nothing in it runs a statement or a ping: it
// is written as each statement ends and each connection attempt returns, by the pool's tracer.
type record struct {
	mu sync.Mutex
	// reporter is the part the pool reports under, nil until the pool is kept, and for the migrator's
	// pool, which nothing probes.
	reporter process.Capabilities
	// reset closes the pool's connections, and holding reports whether the pool holds a connection,
	// idle or in use. Both are nil until the pool is open.
	reset   func()
	holding func() bool
	// fault is set while the latest evidence is a fault.
	fault bool
	// failingSince is the first failed connection attempt since the latest evidence that the database
	// was reachable, zero when there is none.
	failingSince time.Time
	reported     bool
	state        process.State
	reason       string
}

// observe records the evidence an outcome gives, and reports the pool's state when it changed. A
// failed connection attempt is first told whether the pool holds another connection, since one it
// holds can still serve. A statement that shows a fault also closes the pool's other connections,
// which have most likely lost the database too and would otherwise sit idle, counted toward the one
// connection the pool keeps, so the pool then holds none and its maintenance attempts a new
// connection every healthCheckPeriod, whose outcome is the evidence the report clears on.
func (r *record) observe(ctx context.Context, o core.Outcome, err error) {
	if o.Connect && o.Failed {
		o.Holding = r.holds()
	}
	evidence := core.Classify(o)
	if evidence == core.None {
		return
	}
	if reset := r.record(ctx, evidence, o, err); reset != nil && evidence == core.Fault && !o.Connect {
		reset()
	}
}

// record keeps the evidence and reports, under the record's lock, and returns the pool's reset.
func (r *record) record(ctx context.Context, evidence core.Evidence, o core.Outcome, err error) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	switch evidence {
	case core.Reachable:
		r.fault, r.failingSince = false, time.Time{}
	case core.Fault:
		r.fault = true
		if o.Connect && r.failingSince.IsZero() {
			r.failingSince = now
		}
		if r.reporter != nil {
			r.reporter.Logger().DebugContext(ctx, "the database could not serve", "connect", o.Connect, "error", err)
		}
	case core.None:
	}
	r.publish(now)
	return r.reset
}

// attach gives the record the open pool's reset, which a statement that shows a fault calls, and its
// count of the connections it holds, which a failed connection attempt reads.
func (r *record) attach(reset func(), holding func() bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reset, r.holding = reset, holding
}

// holds reports whether the pool holds a connection, false before it is open. It reads the pool's
// statistics outside the record's lock.
func (r *record) holds() bool {
	r.mu.Lock()
	holding := r.holding
	r.mu.Unlock()
	return holding != nil && holding()
}

// start reports the pool under reporter from now on, beginning with its state now.
func (r *record) start(reporter process.Capabilities) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reporter = reporter
	r.publish(time.Now())
}

// publish reports the state the record holds at now, when it differs from the last one reported. It
// runs under the record's lock, so two reports cannot reach the process out of order.
func (r *record) publish(now time.Time) {
	if r.reporter == nil {
		return
	}
	state, reason := process.Ready, ""
	switch {
	case !r.failingSince.IsZero() && now.Sub(r.failingSince) >= noConnectionBound:
		state, reason = process.Failed, disconnected
	case r.fault:
		state, reason = process.NotReady, unreachable
	}
	if r.reported && state == r.state && reason == r.reason {
		return
	}
	r.reported, r.state, r.reason = true, state, reason
	r.reporter.Report(state, reason)
}

// outcome gathers what the classification reads from a statement's or a connection attempt's end.
// conn is the statement's connection, nil for a connection attempt.
func outcome(ctx context.Context, connect bool, conn *pgx.Conn, err error) core.Outcome {
	o := core.Outcome{
		Connect:     connect,
		Failed:      err != nil,
		CallerEnded: ctx.Err() != nil,
		Lost:        conn != nil && conn.IsClosed(),
	}
	var answered interface{ SQLState() string }
	if errors.As(err, &answered) {
		o.Code = answered.SQLState()
	}
	return o
}
