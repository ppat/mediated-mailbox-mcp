package policies

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// ErrUntrustedRead wraps the error of a load whose read of the policy tables did not complete, so
// the rows it holds are not the policy.
var ErrUntrustedRead = errors.New("the read of the policy tables did not complete, so the active policy stays")

// ErrInvalidUpdate wraps the error of a load whose rows do not validate.
var ErrInvalidUpdate = errors.New("the policy tables do not validate, so the active policy stays")

// Snapshot is one immutable policy, with the policy of each account whose own rules it holds
// prepared for the sender classifier. The reload that swaps the policy in prepares each account's, so
// the prepared policies are swapped with it and never outlive it, and each rule's suffixes are
// converted once per load rather than on every classification. The zero value is no policy at all.
type Snapshot struct {
	policy   policy.Snapshot
	prepared map[string]classify.Policy
}

// Loaded reports whether the snapshot came from a valid load.
func (s Snapshot) Loaded() bool { return s.policy.Loaded() }

// For returns the policy account's decisions are made against, prepared for the classifier. An
// account whose own rules the snapshot never read gets the policy that restricts every sender,
// because the base rules alone would leave out every restriction its own rules add. The snapshot
// prepares a policy only for the accounts it read, so such an account gets the zero policy.
func (s Snapshot) For(account string) classify.Policy {
	return s.prepared[account]
}

// loader holds the active snapshot and replaces it on each load that succeeds. Which accounts it
// reads is set when it is built and replaced by setAccounts, and when it loads is its caller's.
type loader struct {
	db      tx.Beginner
	metrics *metrics
	lookups classify.Lookups

	// mu makes reloads take turns, so each swaps against the snapshot the one before it left, and
	// keeps a change of accounts from landing partway through a reload.
	mu       sync.Mutex
	accounts []string
	active   atomic.Pointer[Snapshot]
}

// newLoader returns a loader that reads the base rules and the rules of each of accounts from db,
// prepares each account's policy for the classifier with lookups, and registers the reload series on
// reg. Until its first load succeeds, its snapshot is no policy, which restricts every sender.
func newLoader(db tx.Beginner, accounts []string, reg prometheus.Registerer, lookups classify.Lookups) (*loader, error) {
	read, err := readable(accounts)
	if err != nil {
		return nil, err
	}
	m, err := newMetrics(reg)
	if err != nil {
		return nil, err
	}
	l := &loader{db: db, accounts: read, metrics: m, lookups: lookups}
	l.active.Store(&Snapshot{})
	return l, nil
}

// setAccounts replaces the accounts the loader reads, from its next load on, as a Session whose
// accounts change between its reloads does (ADR-0090). The active snapshot keeps the accounts it was
// read for until then, so an account added here gets the policy that restricts every sender until a
// load reads its rules.
func (l *loader) setAccounts(accounts []string) error {
	read, err := readable(accounts)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.accounts = read
	return nil
}

// readable returns accounts sorted and without repeats, or an error for no account or an empty name.
func readable(accounts []string) ([]string, error) {
	if len(accounts) == 0 {
		return nil, errors.New("the policy loader needs an account, because every read runs in an account's transaction")
	}
	if slices.Contains(accounts, "") {
		return nil, errors.New("an account the policy loader reads has an empty name")
	}
	return slices.Compact(slices.Sorted(slices.Values(accounts))), nil
}

// snapshot returns the active snapshot. A caller keeps the one it took for a whole unit of work, so
// a later load never changes the policy that unit decides against.
func (l *loader) snapshot() Snapshot { return *l.active.Load() }

// reload loads the policy, as swap states, and times the load, every one its caller did not cancel,
// from its call to its outcome (ADR-0125). A body request waits in the shared load for one or two of
// these loads, so its policy wait is one or two observations.
func (l *loader) reload(ctx context.Context) error {
	start := time.Now()
	err := l.swap(ctx)
	if !errors.Is(err, context.Canceled) {
		l.metrics.took(start)
	}
	return err
}

// swap reads the policy tables and makes what it read the active snapshot if the read completed
// and the rows validate. A read whose accounts read different base rules is read once more. Otherwise the active snapshot stays, the reload-failure series reads 1
// until a reload succeeds, and the returned error wraps ErrUntrustedRead or ErrInvalidUpdate. A
// reload its caller cancelled returns an error wrapping context.Canceled and leaves the series as
// it was.
func (l *loader) swap(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.read(ctx)
	if errors.Is(err, errTornBase) {
		rows, err = l.read(ctx)
	}
	if err != nil && errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("the policy reload was cancelled before it finished, so the active policy stays: %w", ctx.Err())
	}
	if err != nil {
		l.metrics.failed()
		return fmt.Errorf("%w: %w", ErrUntrustedRead, err)
	}
	next, outcome := policy.Swap(l.active.Load().policy, rows)
	if !outcome.Accepted() {
		l.metrics.failed()
		return fmt.Errorf("%w: %s", ErrInvalidUpdate, strings.Join(outcome.Problems(), "; "))
	}
	prepared := make(map[string]classify.Policy, len(l.accounts))
	for _, account := range l.accounts {
		prepared[account] = classify.Prepare(next.For(account), l.lookups)
	}
	l.active.Store(&Snapshot{policy: next, prepared: prepared})
	l.metrics.succeeded()
	return nil
}

// read returns every row of the policy the loader's accounts see. Each account's read returns the
// base rules with its own, and the base rules are kept from the first. It returns no rows with any
// error.
func (l *loader) read(ctx context.Context) ([]policy.Row, error) {
	var rows, base []policy.Row
	for i, account := range l.accounts {
		var inherited, own []policy.Row
		err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
			read, err := policyrules.New(t).PolicyRules(ctx)
			if err != nil {
				return fmt.Errorf("reading the rules: %w", err)
			}
			for _, r := range read {
				row := policy.Row{Account: r.AccountID.String, ID: r.RuleID, Class: r.Class, DomainSuffixes: r.DomainSuffix}
				if r.AccountID.Valid {
					own = append(own, row)
				} else {
					inherited = append(inherited, row)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("account %s: %w", account, err)
		}
		if i == 0 {
			base = inherited
			rows = append(rows, base...)
		} else if !slices.EqualFunc(base, inherited, sameRule) {
			return nil, fmt.Errorf("account %s: %w", account, errTornBase)
		}
		rows = append(rows, own...)
	}
	return rows, nil
}

// errTornBase is a read whose accounts read different base rules, because the base policy changed
// between two accounts' reads.
var errTornBase = errors.New("the base rules changed between two accounts' reads")

// sameRule reports whether two rows hold the same rule.
func sameRule(a, b policy.Row) bool {
	return a.Account == b.Account && a.ID == b.ID && a.Class == b.Class && slices.Equal(a.DomainSuffixes, b.DomainSuffixes)
}

// Has reports whether the snapshot was read for the account, so its decisions are made against the
// account's own rules. The Session works an account only once it has (ADR-0090).
func (s Snapshot) Has(account string) bool {
	_, ok := s.prepared[account]
	return ok
}
