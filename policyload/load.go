// Package policyload reads the policy tables into one immutable snapshot through core/policy, makes
// it the active one only when the reload succeeds, and emits the series the reload-failure alarm
// reads (ADR-0041, ADR-0077). policyload/README.md argues why it is a shared library.
//
// # Telling a read it cannot trust from an empty policy
//
// core/policy accepts a policy with no rules, so the rows a reload hands it must be the whole
// policy. A reload reads, for each account in a transaction of its own through db/tx, one statement
// returning the base rules and the account's own rules together, as one consistent set. It trusts
// what it read only when every statement returned without an error, every transaction committed,
// and every account's read returned the same base rules. Whatever rows it then holds, none
// included, are the policy.
//
// That test separates the two cases because each fault in reading that can cut a read short
// surfaces as an error, never as fewer rows. The generated accessor returns the statement's error
// whether it came before the first row or after some of them, a row it cannot decode included. A
// read that never reached the server or lost its connection fails the same way. A transaction whose
// account setting does not hold the account would return no account rows and raise nothing, and
// db/tx fails that transaction before any statement runs. The base rules read in each account's
// transaction are compared, so an edit landing between two accounts' reads fails the reload rather
// than composing accounts from different base policies. So a reload that fails keeps no row it read.
// Its error is returned, the active snapshot stays, and the series the alarm reads says so.
//
// One fault is outside what the loader can see. A migration that drops or narrows the row-level
// security policy showing every account the base rules, policy_rules_base, makes the base rules read
// as absent with no error, and the loader takes that as a policy with no base rules. That is a
// schema change. Review and this package's integration tests, which read the base rules under a
// deployable's role, catch it, and nothing does at run time.
//
// A reload its caller cancelled is not a failed reload. It returns the cancellation, keeps the
// active snapshot, and leaves the alarm's series as it was, so a process shutting down mid-reload
// raises no alarm. A reload whose deadline passed is a failed read like any other, because a caller
// bounding a read that never finishes, against a locked table or a stalled database, would
// otherwise keep the policy stale with no alarm.
package policyload

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// ErrUntrustedRead wraps the error of a reload whose read of the policy tables did not complete, so
// the rows it holds are not the policy.
var ErrUntrustedRead = errors.New("the read of the policy tables did not complete, so the active policy stays")

// ErrInvalidUpdate wraps the error of a reload whose rows do not validate.
var ErrInvalidUpdate = errors.New("the policy tables do not validate, so the active policy stays")

// Snapshot is one immutable policy together with the accounts whose own rules it holds. The zero
// value is no policy at all.
type Snapshot struct {
	policy   policy.Snapshot
	accounts []string
}

// Loaded reports whether the snapshot came from a valid load.
func (s Snapshot) Loaded() bool { return s.policy.Loaded() }

// For returns the policy account's decisions are made against. An account whose own rules the
// snapshot never read gets the policy that restricts every sender, because the base rules alone
// would leave out every restriction its own rules add.
func (s Snapshot) For(account string) policy.Composed {
	if _, read := slices.BinarySearch(s.accounts, account); !read {
		return policy.Composed{}
	}
	return s.policy.For(account)
}

// Loader holds the active snapshot and replaces it on each reload that succeeds. Which accounts it
// reads is fixed when it is built, and when it reloads is its caller's.
type Loader struct {
	db       tx.Beginner
	accounts []string
	metrics  *metrics

	// mu makes reloads take turns, so each swaps against the snapshot the one before it left.
	mu     sync.Mutex
	active atomic.Pointer[Snapshot]
}

// New returns a Loader that reads the base rules and the rules of each of accounts from db, and
// registers the reload-failure series on reg. Until its first reload succeeds, its snapshot is no
// policy, which restricts every sender.
func New(db tx.Beginner, accounts []string, reg prometheus.Registerer) (*Loader, error) {
	if len(accounts) == 0 {
		return nil, errors.New("the policy loader needs an account, because every read runs in an account's transaction")
	}
	if slices.Contains(accounts, "") {
		return nil, errors.New("an account the policy loader reads has an empty name")
	}
	m, err := newMetrics(reg)
	if err != nil {
		return nil, err
	}
	l := &Loader{db: db, accounts: slices.Compact(slices.Sorted(slices.Values(accounts))), metrics: m}
	l.active.Store(&Snapshot{})
	return l, nil
}

// Snapshot returns the active snapshot. A caller keeps the one it took for a whole unit of work, so
// a later reload never changes the policy that unit decides against.
func (l *Loader) Snapshot() Snapshot { return *l.active.Load() }

// Reload reads the policy tables and makes what it read the active snapshot if the read completed
// and the rows validate. Otherwise the active snapshot stays, the reload-failure series reads 1
// until a reload succeeds, and the returned error wraps ErrUntrustedRead or ErrInvalidUpdate. A
// reload its caller cancelled returns an error wrapping context.Canceled and leaves the series as
// it was.
func (l *Loader) Reload(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.read(ctx)
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
	l.active.Store(&Snapshot{policy: next, accounts: l.accounts})
	l.metrics.succeeded()
	return nil
}

// read returns every row of the policy the loader's accounts see. Each account's read returns the
// base rules with its own, and the base rules are kept from the first. It returns no rows with any
// error.
func (l *Loader) read(ctx context.Context) ([]policy.Row, error) {
	var rows, base []policy.Row
	for i, account := range l.accounts {
		var inherited, own []policy.Row
		err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
			read, err := policyrules.New(t).PolicyRules(ctx, account)
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
			return nil, fmt.Errorf("account %s: the base rules changed between two accounts' reads", account)
		}
		rows = append(rows, own...)
	}
	return rows, nil
}

// sameRule reports whether two rows hold the same rule.
func sameRule(a, b policy.Row) bool {
	return a.Account == b.Account && a.ID == b.ID && a.Class == b.Class && slices.Equal(a.DomainSuffixes, b.DomainSuffixes)
}
