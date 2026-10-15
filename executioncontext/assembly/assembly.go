// Package assembly builds a process's one execution context, what every component of the process
// that calls a provider executes with, and runs its one reload. It holds the keyring, the account
// snapshot and its loader, the sources of the account sessions, so an account has one token source
// and one rotation write-back in the process, the policy snapshot and its loader, each account's
// lowered rate target, and the scanner. Its statements run under the role of the pool its composition
// root hands it, the API server's own in the API server and delta sync's in the worker (ADR-0090,
// ADR-0118). Each component sees the sessions through a holder of its own, with its own provider
// adapters and the role its own attempts are recorded under, and builds each unit's rate limiter on
// its own pool, so its series of provider calls and leases stay its own (ADR-0117). The UI never
// receives it. Its case is argued in executioncontext/README.md.
package assembly

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// ErrPolicyNotLoaded wraps a reload's error when the accounts loaded and the policy did not. The
// accounts and their targets the reload read are in force all the same, and the policy loader keeps
// the policy it had (ADR-0041).
var ErrPolicyNotLoaded = errors.New("the policy was not loaded")

// ErrNotLoaded is a reload's state before any load of the accounts has succeeded.
var ErrNotLoaded = errors.New("the execution context has not loaded its accounts yet")

// Needs are what the execution context is built from.
type Needs struct {
	// Pool connects as the role the context's statements run under.
	Pool *pgxpool.Pool
	// Keys open the accounts' sealed credentials and the OAuth clients' secrets (ADR-0081).
	Keys *open.Keyring
	// ThroughClient names the providers that authenticate through an OAuth client, which the root knows
	// from the adapters it links (ADR-0106).
	ThroughClient []string
	// Scanner is the process's one scanner, whose revision every verdict, mask and serve-time check in
	// the process records (ADR-0120).
	Scanner scan.Scanner
	// Lookups are the classifier's domain functions each account's policy is prepared with.
	Lookups classify.Lookups
	// ReloadInterval is how often Run reloads.
	ReloadInterval time.Duration
	// Registerer is where the policy loader's series are registered, once in the process.
	Registerer prometheus.Registerer
	Logger     *slog.Logger
}

// reloadDurationName is the series timing each reload of the execution context, K12 of ADR-0125.
const reloadDurationName = "mediated_mailbox_executioncontext_reload_duration_seconds"

// Context is a process's execution context.
type Context struct {
	n Needs
	// took times each reload its caller did not cancel (ADR-0125).
	took     prometheus.Histogram
	loader   *accountload.Loader
	sessions *session.Holder
	// policies is built at the first reload that lists an account, since a policy loader reads at
	// least one, and never replaced.
	policies atomic.Pointer[policyload.Loader]

	// reloading makes reloads take turns.
	reloading sync.Mutex
	latest    atomic.Pointer[Reloaded]

	observersMu sync.Mutex
	observers   []func(context.Context)
}

// Reloaded is what the latest reload left in force.
type Reloaded struct {
	snapshot *accountload.Snapshot
	targets  map[string]target
	loaded   bool
	err      error
}

// target is an account's lowered rate target as the reload read it.
type target struct {
	fraction float64
	lowered  bool
	err      error
}

// Snapshot is the account snapshot in force, which serves no account before the first load.
func (r *Reloaded) Snapshot() *accountload.Snapshot { return r.snapshot }

// Loaded reports whether a load of the accounts has succeeded.
func (r *Reloaded) Loaded() bool { return r.loaded }

// Err is the latest reload's error, ErrNotLoaded before any reload, an error wrapping
// ErrPolicyNotLoaded when only the policy failed, and the read's error when the accounts' read
// failed, which left in force what the reload before it read.
func (r *Reloaded) Err() error { return r.err }

// New returns the execution context built from n, with the duration of its reloads registered on
// n.Registerer. It loads nothing until it is reloaded.
func New(n Needs) (*Context, error) {
	took := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    reloadDurationName,
		Help:    "How long each reload of the process's execution context took, its accounts, their credentials, their lowered targets and their policy, succeeded or failed.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
	})
	if err := n.Registerer.Register(took); err != nil {
		return nil, err
	}
	loader := accountload.New(n.Pool, n.Keys, n.Logger, n.ThroughClient)
	c := &Context{
		n: n, loader: loader, took: took,
		sessions: session.New(session.Config{Loader: loader, Logger: n.Logger, Hold: true}),
	}
	c.latest.Store(&Reloaded{snapshot: &accountload.Snapshot{}, err: ErrNotLoaded})
	return c, nil
}

// Pool returns the pool the context's statements run on, which a composition root's test holds to
// the role the root chose for them (ADR-0118).
func (c *Context) Pool() *pgxpool.Pool { return c.n.Pool }

// Scanner returns the process's one scanner.
func (c *Context) Scanner() scan.Scanner { return c.n.Scanner }

// Loader returns the account loader, through which delta sync re-seals what it opened (ADR-0092).
func (c *Context) Loader() *accountload.Loader { return c.loader }

// Latest returns what the latest reload left in force.
func (c *Context) Latest() *Reloaded { return c.latest.Load() }

// Snapshot returns the account snapshot in force, the one every unit of work takes at its start. It is
// the loader's, so a credential a unit read again after a refusal reaches the units after it before
// the next reload (ADR-0090).
func (c *Context) Snapshot() *accountload.Snapshot { return c.loader.Snapshot() }

// Sessions returns a holder through which a component opens its accounts' sessions over the
// process's sources, with its own connectors' ports, recording each session's latest authentication
// attempt through db, the component's own pool (ADR-0097, ADR-0117).
func (c *Context) Sessions(connectors map[string]session.Connector, db tx.Beginner) *session.Holder {
	return c.sessions.Through(connectors, db)
}

// OnReload has observe called after every reload whose accounts loaded, the policy's failure
// included, so a component follows the accounts in force. Observers run on the reload's goroutine, in
// the order they were added.
func (c *Context) OnReload(observe func(context.Context)) {
	c.observersMu.Lock()
	defer c.observersMu.Unlock()
	c.observers = append(c.observers, observe)
}

// Reload loads the account snapshot, reads the lowered rate target of every connected account it
// lists and loads the policy of every account it lists, then tells each observer. A read of the
// accounts that fails keeps in force what the reload before it read and returns the error. A policy
// load that fails returns an error wrapping ErrPolicyNotLoaded, with the accounts and their targets in
// force all the same, so an account no longer listed is dropped, and the policy loader keeps the
// policy it had (ADR-0090, ADR-0041). Reloads take turns, and each one its caller did not cancel is
// timed from its start to its outcome (ADR-0125).
func (c *Context) Reload(ctx context.Context) error {
	c.reloading.Lock()
	defer c.reloading.Unlock()
	start := time.Now()
	err := c.reload(ctx)
	if !errors.Is(err, context.Canceled) {
		c.took.Observe(time.Since(start).Seconds())
	}
	return err
}

// reload is one reload, as Reload states.
func (c *Context) reload(ctx context.Context) error {
	previous := c.latest.Load()
	if err := c.loader.Load(ctx); err != nil {
		err = fmt.Errorf("loading the accounts: %w", err)
		c.latest.Store(&Reloaded{snapshot: previous.snapshot, targets: previous.targets, loaded: previous.loaded, err: err})
		return err
	}
	snapshot := c.loader.Snapshot()
	targets := c.readTargets(ctx, snapshot)
	var err error
	if policyErr := c.loadPolicy(ctx, idsOf(snapshot)); policyErr != nil {
		err = fmt.Errorf("%w: %w", ErrPolicyNotLoaded, policyErr)
	}
	c.latest.Store(&Reloaded{snapshot: snapshot, targets: targets, loaded: true, err: err})
	c.observersMu.Lock()
	observers := c.observers
	c.observersMu.Unlock()
	for _, observe := range observers {
		observe(ctx)
	}
	return err
}

// firstRetry is how soon a reload is tried again while no load has succeeded, so a process started
// before its database answers serves soon after it does.
const firstRetry = 5 * time.Second

// Run reloads until ctx ends, at once when no load has succeeded yet, then on every tick of the
// reload interval, and every firstRetry while no load has succeeded. A failed reload is logged and
// keeps in force what was, and Run returns nil once ctx ends.
func (c *Context) Run(ctx context.Context) error {
	wait := time.Duration(0)
	if c.Latest().Loaded() {
		wait = c.n.ReloadInterval
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		if err := c.Reload(ctx); err != nil && ctx.Err() == nil {
			c.n.Logger.Error("the execution context was not reloaded in full, so what it held stays in force", "error", err)
		}
		wait = c.n.ReloadInterval
		if !c.Latest().Loaded() {
			wait = min(firstRetry, c.n.ReloadInterval)
		}
		timer.Reset(wait)
	}
}

// loadPolicy sets the accounts the policy loader reads and has it load. With no account there is no
// policy to load.
func (c *Context) loadPolicy(ctx context.Context, accounts []string) error {
	if len(accounts) == 0 {
		return nil
	}
	policies := c.policies.Load()
	if policies == nil {
		built, err := policyload.New(c.n.Pool, accounts, c.n.Registerer, c.n.Lookups)
		if err != nil {
			return err
		}
		c.policies.Store(built)
		policies = built
	} else if err := policies.SetAccounts(accounts); err != nil {
		return err
	}
	return policies.Reload(ctx)
}

// ReloadPolicy is one load of the policy, of the accounts the latest reload listed, which the API
// server's coalesced load runs before each body request decides (ADR-0099). Before the first reload
// that lists an account there is nothing to load. The policy loader takes its reloads in turn, so
// a load this starts is never overtaken by an older one the reload runs.
func (c *Context) ReloadPolicy(ctx context.Context) error {
	policies := c.policies.Load()
	if policies == nil {
		return nil
	}
	return policies.Reload(ctx)
}

// PolicySnapshot returns the policy snapshot in force, which a unit of work takes at its start and
// keeps. Before any load it is no policy, which restricts every sender (ADR-0041).
func (c *Context) PolicySnapshot() policyload.Snapshot {
	policies := c.policies.Load()
	if policies == nil {
		return policyload.Snapshot{}
	}
	return policies.Snapshot()
}

// Policy returns the policy the account's messages are decided against in the policy snapshot in
// force.
func (c *Context) Policy(account string) classify.Policy {
	return c.PolicySnapshot().For(account)
}

// Limiter returns the rate limiter a unit of work for the account spends from, at the lowered target
// the latest reload read for the account, if it sets one, against ceiling, its provider's declared
// budget per second (ADR-0024). Its leases are taken on db, the component's own pool, and recorded in
// metrics, the component's own series (ADR-0025, ADR-0117). An account whose target could not be read,
// or is out of the range ADR-0024 allows, has no limiter, so its work stops rather than spend at a
// target nobody set.
func (c *Context) Limiter(db tx.Beginner, account string, ceiling float64, metrics *lease.Metrics) (*lease.Limiter, error) {
	t, ok := c.latest.Load().targets[account]
	if !ok {
		return nil, fmt.Errorf("no lowered target was read for account %s, since the reload in force does not list it as connected", account)
	}
	if t.err != nil {
		return nil, t.err
	}
	targets := map[string]float64{}
	if t.lowered {
		targets[account] = t.fraction
	}
	limiter, err := lease.NewWithTargets(db, ceiling, targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
}

// readTargets reads the lowered target of every connected account of snapshot. A read that fails is
// that account's alone, logged and kept for its limiter to return.
func (c *Context) readTargets(ctx context.Context, snapshot *accountload.Snapshot) map[string]target {
	out := map[string]target{}
	for _, a := range snapshot.Accounts() {
		if !a.Connected() {
			continue
		}
		var t target
		err := tx.Run(ctx, c.n.Pool, a.ID(), func(q pgx.Tx) error {
			lowered, err := accountstate.New(q).LoweredTarget(ctx)
			if err != nil {
				return err
			}
			t = target{fraction: float64(lowered.Float32), lowered: lowered.Valid}
			return nil
		})
		if err != nil {
			t = target{err: fmt.Errorf("reading the lowered target of account %s: %w", a.ID(), err)}
			c.n.Logger.Warn("reading an account's lowered target failed, so its work stops until a reload reads it", "account", a.ID(), "error", err)
		}
		out[a.ID()] = t
	}
	return out
}

// idsOf returns the identifiers of a snapshot's accounts, in order.
func idsOf(snapshot *accountload.Snapshot) []string {
	var ids []string
	for _, a := range snapshot.Accounts() {
		ids = append(ids, a.ID())
	}
	return ids
}
