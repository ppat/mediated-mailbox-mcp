// Package policies is each account's policy as a process's Session holds it, the base policy
// included, the load concurrent body requests share, and the reload-failure alarm (ADR-0041,
// ADR-0077, ADR-0099). It reads the policy tables into one immutable snapshot through core/policy,
// makes it the active one only when the load succeeds, and prepares each account's policy for the
// sender classifier in the load that swaps it in. Which accounts it reads, and when it reloads, are
// the Session's. Its case is argued in process/README.md.
//
// # Telling a read it cannot trust from an empty policy
//
// core/policy accepts a policy with no rules, so the rows a load hands it must be the whole policy.
// A load reads, for each account in a transaction of its own through db/tx, one statement returning
// the base rules and the account's own rules together, as one consistent set. It trusts what it read
// only when every statement returned without an error, every transaction committed, and every
// account's read returned the same base rules. Whatever rows it then holds, none included, are the
// policy.
//
// That test separates the two cases because each fault in reading that can cut a read short
// surfaces as an error, never as fewer rows. The generated accessor returns the statement's error
// whether it came before the first row or after some of them, a row it cannot decode included. A
// read that never reached the server or lost its connection fails the same way. A transaction whose
// account setting does not hold the account would return no account rows and raise nothing, and
// db/tx fails that transaction before any statement runs. The base rules read in each account's
// transaction are compared, so an edit landing between two accounts' reads never composes accounts
// from different base policies. Such a load reads every account once more at once, and fails only
// when the second read's accounts disagree too, so an edit to the base policy raises the alarm only
// when edits keep landing through both reads (ADR-0114). The second read is the last, so edits
// landing without pause fail the load rather than hold it in a loop. So a load that fails keeps no
// row it read. Its error is returned, the active snapshot stays, and the series the alarm reads says
// so.
//
// A load its caller cancelled is not a failed load. It returns the cancellation, keeps the active
// snapshot, and leaves the alarm's series as it was, so a process shutting down mid-load raises no
// alarm. A load whose deadline passed is a failed read like any other, because a caller bounding a
// read that never finishes, against a locked table or a stalled database, would otherwise keep the
// policy stale with no alarm.
package policies

import (
	"context"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process/domains"
)

// Policies is each account's policy a Session holds. Its zero value is not usable. Build it with New.
type Policies struct {
	db      tx.Beginner
	reg     prometheus.Registerer
	lookups classify.Lookups
	// loader is built at the first load that names an account, since a loader reads at least one and
	// registers the reload series when it is built, so a process that holds no account carries none.
	// It is never replaced.
	loader atomic.Pointer[loader]
	fresh  *shared
}

// New returns the policies of a Session, read through db, with the reload series registered on reg
// at the first load that names an account and the load concurrent body requests share running on the
// goroutines start starts, the process's Go (ADR-0099, ADR-0142). Each account's policy is prepared
// for the sender classifier with the lookups every program classifies with, the IDNA conversions and
// the public suffix list. Until a load succeeds it holds no policy, which restricts every sender.
func New(db tx.Beginner, reg prometheus.Registerer, start Starter) *Policies {
	p := &Policies{
		db: db, reg: reg,
		lookups: domains.Lookups(),
	}
	p.fresh = newShared(start, p.reloadCurrent)
	return p
}

// Load reads the policy of the accounts the Session has put in force, which the loads after it read
// until the next Load. With no account there is no policy to load. A load that fails keeps the active
// snapshot and returns an error wrapping ErrUntrustedRead or ErrInvalidUpdate. The Session calls it
// from one reload at a time.
func (p *Policies) Load(ctx context.Context, accounts []string) error {
	if len(accounts) == 0 {
		return nil
	}
	l := p.loader.Load()
	if l == nil {
		built, err := newLoader(p.db, accounts, p.reg, p.lookups)
		if err != nil {
			return err
		}
		p.loader.Store(built)
		l = built
	} else if err := l.setAccounts(accounts); err != nil {
		return err
	}
	return l.reload(ctx)
}

// Fresh waits for a load that starts after it was called, of the accounts the latest Load read, and
// returns its error, so a body request decides under a policy at least as new as its arrival. The
// callers waiting when a load starts share it (ADR-0099). Before the first Load that names an account
// there is nothing to load.
func (p *Policies) Fresh(ctx context.Context) error { return p.fresh.load(ctx) }

// reloadCurrent is one load of the accounts the latest Load read. The loader takes its loads in turn,
// so a load Fresh starts is never overtaken by an older one a reload runs.
func (p *Policies) reloadCurrent(ctx context.Context) error {
	l := p.loader.Load()
	if l == nil {
		return nil
	}
	return l.reload(ctx)
}

// Snapshot returns the policy snapshot in force, which a unit of work takes at its start and keeps.
// Before any load it is no policy, which restricts every sender (ADR-0041).
func (p *Policies) Snapshot() Snapshot {
	l := p.loader.Load()
	if l == nil {
		return Snapshot{}
	}
	return l.snapshot()
}
