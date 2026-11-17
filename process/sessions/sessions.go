// Package sessions is the Session, all the user state a process holds, which is the accounts in
// force, each account's policy and each account's connection to its provider, with its one reload,
// the order of the reload, its failure rules, and who is told after it (ADR-0090, ADR-0142). It is
// built once per process, from the process, and its reload is the Session's own upkeep on the
// process's scheduler. Its case is argued in process/README.md.
//
// It imports no credential and no provider code. The connections import it and attach to it, once,
// as its third member, and it reaches them only through the small interface Connections. So code that
// holds the Session and not the connections cannot reach a credential.
package sessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/accounts"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/policies"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// ErrPolicyNotLoaded wraps a reload's error when the accounts and the connections loaded and the
// policy did not. The accounts and the connections the reload read are in force all the same, and the
// policy keeps what it had (ADR-0041).
var ErrPolicyNotLoaded = errors.New("the policy was not loaded")

// reloadDurationName is the series timing each reload of the Session, K12 of ADR-0125.
const reloadDurationName = "mediated_mailbox_executioncontext_reload_duration_seconds"

// firstRetry is how soon a reload is tried again while no reload has loaded the accounts, their
// connections and their policy, so a process whose first reload fails after its start's schema check
// becomes ready soon after one succeeds (ADR-0090, ADR-0140).
const firstRetry = 5 * time.Second

// notReady is the reason the Session reports until a reload has loaded everything once.
const notReady = "no reload has loaded the accounts, their connections and their policy yet"

// Connections is what the Session asks of the connections attached to it, its third member.
type Connections interface {
	// Load loads the connections of the accounts listed. A read it cannot trust returns an error and
	// leaves what it held in force, and an account it cannot connect is left without a connection
	// while the others load.
	Load(ctx context.Context, listed []accounts.Account) error
	// Connected reports whether the account has a connection a unit of work may begin on.
	Connected(account string) bool
}

// held is the slot the process keeps its one Session in.
var held = process.NewOnePerProcess[*Session]("session")

// Session is all the user state one process holds. It is safe for concurrent use.
type Session struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	report   func(state process.State, reason string)
	policies *policies.Policies
	took     prometheus.Histogram
	interval time.Duration

	// reloading makes reloads take turns.
	reloading sync.Mutex
	// inForce is the list of accounts in force, nil before the first reload that read one.
	inForce atomic.Pointer[[]accounts.Account]
	// loaded is set once a reload has loaded the list, the connections where the process has them,
	// and the policy.
	loaded atomic.Bool

	membersMu   sync.Mutex
	connections Connections
	after       []func()
}

// Get returns the process's Session, built the first time any part asks, from the process, with the
// process's own pool, logger and registerer, and the same one on every later call (ADR-0142). Its
// first reload runs once the process serves, on the process's scheduler, and then on the interval c's
// accounts part sets. It reports not ready until a reload has loaded everything once.
func Get(ctx context.Context, p process.Capabilities, c config.HasAccounts) (*Session, error) {
	return held.Get(ctx, p, func(ctx context.Context, shared process.Capabilities) (*Session, func(), error) {
		pool, err := database.Get(ctx, shared, c)
		if err != nil {
			return nil, nil, err
		}
		s := &Session{
			pool: pool, logger: shared.Logger(), report: shared.Report,
			policies: policies.New(pool, shared.Metrics(), shared.Go),
			took: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    reloadDurationName,
				Help:    "How long each reload of the process's Session took, its accounts, their connections, their lowered targets and their policy, succeeded or failed.",
				Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
			}),
			interval: time.Duration(c.AccountsPart().AccountReloadInterval),
		}
		if err := shared.Metrics().Register(s.took); err != nil {
			return nil, nil, err
		}
		if err := process.Upkeep(shared, s.upkeep()); err != nil {
			shared.Metrics().Unregister(s.took)
			return nil, nil, err
		}
		shared.Report(process.NotReady, notReady)
		return s, nil, nil
	})
}

// upkeep is the Session's reload as the process's scheduler runs it, at once when the process serves
// and then on the reload interval. While no reload has loaded everything a failed reload is tried
// again after five seconds, or the interval when it is shorter. Once one has, a failed reload keeps
// in force what was, is logged, and is tried again at the next interval, so it is answered as not due
// rather than failed, whose backoff would try it sooner (ADR-0090).
func (s *Session) upkeep() schedule.Job {
	retry := min(firstRetry, s.interval)
	return schedule.Job{
		Interval: s.interval,
		Backoff:  schedule.Backoff{Base: retry, Max: retry},
		Logger:   s.logger,
		Run: func(ctx context.Context, _ schedule.Ask) error {
			err := s.Reload(ctx)
			if err == nil || ctx.Err() != nil || !s.loaded.Load() {
				return err
			}
			s.logger.Error("the Session was not reloaded in full, so what it held stays in force", "error", err)
			return schedule.ErrNotDue
		},
	}
}

// Attach makes c the Session's third member, the connections, which the reloads after it load for
// the list they read. The connections attach themselves, once, when they are built, and the go vet
// analyser rule on its callers holds every other package from calling it (ADR-0071). A second attach
// is refused.
func (s *Session) Attach(c Connections) error {
	s.membersMu.Lock()
	defer s.membersMu.Unlock()
	switch {
	case c == nil:
		return errors.New("no connections to attach")
	case s.connections != nil:
		return errors.New("connections are attached to the Session already")
	}
	s.connections = c
	return nil
}

// members returns the connections attached, nil for none, and those told after each reload.
func (s *Session) members() (Connections, []func()) {
	s.membersMu.Lock()
	defer s.membersMu.Unlock()
	return s.connections, s.after
}

// AfterReload has tell called after every reload whose accounts and connections loaded, the
// policy's failure included, so a scheduler's plan or the API server's registry follows the accounts
// in force. They are called on the reload's goroutine, in the order they were added.
func (s *Session) AfterReload(tell func()) {
	s.membersMu.Lock()
	defer s.membersMu.Unlock()
	s.after = append(s.after, tell)
}

// Reload reloads the Session once, as its upkeep does on the interval. Its order is written once.
// It reads the list of accounts, loads the connections for it, puts the list in force, loads the
// policy for it, and tells those who asked.
//
//   - A read of the list that fails keeps in force what is, loads nothing, tells no one, and returns
//     the error.
//   - A load of the connections that fails keeps in force what is, the list included, loads nothing
//     more, tells no one, and returns the error.
//   - A load of the policy that fails leaves the list and the connections in force, so an account no
//     longer listed is dropped, keeps the policy it had, tells those who asked, and returns an error
//     wrapping ErrPolicyNotLoaded (ADR-0041, ADR-0090).
//
// Reloads take turns, and each one its caller did not cancel is timed from its start to its outcome
// (ADR-0125).
func (s *Session) Reload(ctx context.Context) error {
	s.reloading.Lock()
	defer s.reloading.Unlock()
	start := time.Now()
	err := s.reload(ctx)
	if !errors.Is(err, context.Canceled) {
		s.took.Observe(time.Since(start).Seconds())
	}
	return err
}

// reload is one reload, as Reload states.
func (s *Session) reload(ctx context.Context) error {
	listed, err := accounts.Read(ctx, s.pool)
	if err != nil {
		return fmt.Errorf("reading the accounts: %w", err)
	}
	connections, after := s.members()
	if connections != nil {
		if err := connections.Load(ctx, listed); err != nil {
			return fmt.Errorf("loading the connections: %w", err)
		}
	}
	s.inForce.Store(&listed)
	if err = s.policies.Load(ctx, ids(listed)); err != nil {
		err = fmt.Errorf("%w: %w", ErrPolicyNotLoaded, err)
	} else if !s.loaded.Swap(true) {
		s.report(process.Ready, "")
	}
	for _, tell := range after {
		tell()
	}
	return err
}

// ids returns the identifiers of the accounts, in their order.
func ids(listed []accounts.Account) []string {
	out := make([]string, 0, len(listed))
	for _, a := range listed {
		out = append(out, a.ID)
	}
	return out
}

// Accounts returns the accounts in force, in identifier order, none before the first reload that read
// the list.
func (s *Session) Accounts() []accounts.Account {
	if listed := s.inForce.Load(); listed != nil {
		return slices.Clone(*listed)
	}
	return nil
}

// Listed returns the account in force with the identifier, and whether the list in force holds it. No
// unit of work begins on an account it does not hold.
func (s *Session) Listed(account string) (accounts.Account, bool) {
	listed := s.inForce.Load()
	if listed == nil {
		return accounts.Account{}, false
	}
	i, ok := slices.BinarySearchFunc(*listed, account, func(a accounts.Account, id string) int {
		switch {
		case a.ID < id:
			return -1
		case a.ID > id:
			return 1
		default:
			return 0
		}
	})
	if !ok {
		return accounts.Account{}, false
	}
	return (*listed)[i], true
}

// Policy returns the policy the account's messages are decided against in the policy in force, the
// base policy included. An account whose rules were never read gets the policy that restricts every
// sender (ADR-0041).
func (s *Session) Policy(account string) classify.Policy {
	return s.policies.Snapshot().For(account)
}

// FreshPolicy waits for a load of the policy that starts after it was called, which the callers
// waiting when it starts share, so a body request decides under a policy at least as new as its
// arrival (ADR-0099). A load that fails keeps the policy in force and returns its error.
func (s *Session) FreshPolicy(ctx context.Context) error { return s.policies.Fresh(ctx) }

// Workable returns the accounts a job kind that calls a provider may work on now, in identifier
// order, each listed, connected and with its policy loaded, so no newly listed account is worked
// before its policy has loaded (ADR-0090).
func (s *Session) Workable() []string {
	connections, _ := s.members()
	if connections == nil {
		return nil
	}
	return s.with(func(id string) bool { return connections.Connected(id) })
}

// AccountsWithPolicy returns the accounts a job kind that calls no provider may work on now, in
// identifier order, each listed and with its policy loaded.
func (s *Session) AccountsWithPolicy() []string {
	return s.with(func(string) bool { return true })
}

// with returns the accounts in force whose policy is loaded and for which also holds.
func (s *Session) with(also func(string) bool) []string {
	snapshot := s.policies.Snapshot()
	var out []string
	for _, a := range s.Accounts() {
		if snapshot.Has(a.ID) && also(a.ID) {
			out = append(out, a.ID)
		}
	}
	return out
}
