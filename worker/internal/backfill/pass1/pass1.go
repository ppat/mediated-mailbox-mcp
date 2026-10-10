// Package pass1 runs backfill's first pass over one account, enumerating every page of the mailbox's
// metadata and making each page durable with its checkpoint in one transaction (ADR-0017). Once the
// enumeration has ended, it fetches again by identifier the stored subjects masked under another
// scanner, a call at a time, each call durable with the run's progress in one transaction, and ends
// only once no stored subject is stale (ADR-0120). The pass's decisions come from
// worker/internal/core/backfill/pass1, and this package enacts them.
//
// A Pass is opened once per run and asked for its next step until it is done. Opening it is the
// recovery path. It reads the account's latest run and resumes from its checkpoint, so a run killed
// at any point repeats at most the one page or call that was not yet durable. Everything a page adds
// reaches the Store in one call, which makes the page's rows, its masking events, its senders'
// statistics and the run's checkpoint durable together or not at all.
package pass1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// Event is one event on a run's timeline, beside the start, progress, finish and failure events the
// Store records with the change they mark (ADR-0117).
type Event struct {
	// Kind is backoff or retry.
	Kind string
	// Page is the page the event concerns, counted from one.
	Page int
	// Detail is the provider's error text, never a body.
	Detail string
}

// Failure is one page the provider failed, as a run records it once it knows what became of the page.
type Failure struct {
	// Page is the page, counted from one.
	Page int
	// Class is the error class, throttled, provider_error, gone, validation or authentication.
	Class string
	// Summary is the provider's last error text, never a body.
	Summary string
	// Attempts counts every attempt at the page, the one that recovered it included.
	Attempts    int
	First, Last time.Time
	// Disposition is recovered for a page a later attempt made durable, recorded in the transaction
	// that makes it durable, and abandoned for one the run failed on.
	Disposition string
}

// Refetched is what one call fetching stale subjects again brought back, as the shell hands it to the
// Store.
type Refetched struct {
	// Remasked are the subjects masked again from the subject the provider returned, and Gone the
	// subjects masked whole because the provider left their message out of its answer.
	Remasked, Gone []index.Message
	// Recovered holds an item for each message of a call that earlier attempts failed and this one
	// answered, recorded with the call's result.
	Recovered []Item
	// Now is the instant a gone message is recorded at.
	Now time.Time
}

// Committed is what a page's commit made durable.
type Committed struct {
	Progress core.Progress
	// Unclassified counts the messages the commit added whose sender could not be classified, and
	// leaves out the ones the index already held.
	Unclassified int
}

// Store is where a pass keeps the index and its runs. Each method is one transaction.
type Store interface {
	// State returns whether the account's pass has ended, how many stored subjects were masked under
	// another scanner than the one stamped s, any of which makes the pass due again, and its latest
	// recorded run of the pass.
	State(ctx context.Context, account string, s index.Stamp) (ended bool, stale int, latest core.Latest[core.Progress], err error)
	// Start records the run runID as it starts from start, recording the run it resumes as failed
	// first when start abandons it, and the pass as not ended when start reopens it, with a start
	// event, or a resume event naming the checkpoint page.
	Start(ctx context.Context, account, runID string, start core.Start[core.Progress]) error
	// Commit makes one page durable. It adds each of the page's messages the index does not hold yet,
	// masks again the subject of each it holds whose subject was masked under another scanner than
	// the message's, records a masking event for each mask on the subject of a message it added or
	// masked again and for no other, rebuilds the statistics of the page's senders, records recovered
	// when earlier attempts at the page failed, and records the progress advance returns for the
	// numbers of messages it added and masked again, with a progress event, all in one transaction.
	Commit(ctx context.Context, account, runID string, page index.Page, recovered *Failure, advance func(added, remasked int) core.Progress) (Committed, error)
	// Stale returns up to n of the account's stored subjects masked under another scanner than the
	// one stamped s, in the order of their identifiers.
	Stale(ctx context.Context, account string, s index.Stamp, n int) ([]core.Stored, error)
	// Refetch makes one call fetching stale subjects again durable. It stores each subject r holds
	// whose stored one is still masked under another scanner, with a masking event for each mask,
	// records each of r's gone messages whose subject it stored as a failed item of class gone, and
	// r's recovered items, and records the progress advance returns for the number of subjects it
	// stored and the stored subjects then still masked under another scanner than the one stamped s,
	// with a progress event, all in one transaction.
	Refetch(ctx context.Context, account, runID string, s index.Stamp, r Refetched, advance func(fetched, stale int) core.Progress) (core.Progress, error)
	// Finish records that the run ended the pass, when no stored subject is masked under another
	// scanner than the one stamped s. It then sets the account's completion flag and records the run as
	// succeeded with a finish event, all in one transaction. It reports whether it ended the pass, and
	// leaves everything as it was when a subject is stale.
	Finish(ctx context.Context, account, runID string, s index.Stamp) (bool, error)
	// Fail records the run as failed with its last error, and a failure event.
	Fail(ctx context.Context, account, runID, cause string) error
	// Event adds an event to the run's timeline.
	Event(ctx context.Context, account, runID string, e Event) error
	// Failure records one page the run failed on.
	Failure(ctx context.Context, account, runID string, f Failure) error
	// Items records the items the run failed on, one for each message of a call it abandoned.
	Items(ctx context.Context, account, runID string, items []Item) error
}

// Fetch returns the page of the account's enumeration that token names, the empty token naming the
// first. Its errors wrap the Provider Port's.
type Fetch func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error)

// Metadata returns the metadata of the messages ids names in one call, leaving out a message the
// mailbox no longer holds, as the Provider Port's metadata read does. Its errors wrap the Provider
// Port's.
type Metadata func(ctx context.Context, ids []string) ([]mail.MessageMetadata, error)

// Deps are what a Pass runs with.
type Deps struct {
	Store Store
	Fetch Fetch
	// Metadata fetches stale subjects again, and PerCall is how many identifiers one of its calls
	// names, the most whose cost fits the hard cap (ADR-0023, ADR-0120).
	Metadata Metadata
	PerCall  int
	// Policy returns the account's policy in the active snapshot, which each page takes at its entry,
	// so a policy edit reaches the pages after it (ADR-0041, ADR-0119). Scanner is the scanner subjects
	// are masked with, and Lookups the classifier's domain functions.
	Policy  func() policy.Composed
	Scanner scan.Scanner
	Lookups classify.Lookups
	// RunID returns a new run's identifier, a short opaque string.
	RunID func() string
	// Now is the clock a failure's first and last attempts are read from.
	Now func() time.Time
}

// Pass is one run of the first pass over one account.
type Pass struct {
	deps    Deps
	account string
	run     string
	at      core.Progress
	done    bool
	// resumed is the token the run resumed from, until the run makes a page durable.
	resumed   mail.PageToken
	restarted bool
}

// Step is what one call of Next did.
type Step struct {
	// Done is set once the pass has ended for the account, or was found ended and not due again when
	// the run opened.
	Done bool
	// Unclassified counts the messages of the page made durable whose sender could not be classified.
	Unclassified int
}

// Open starts a run of the pass over the account, resuming its latest run when that run stopped
// before the pass ended, and starting its enumeration over when that run enumerated under another
// scanner and its enumeration has not ended. A pass that has ended opens done and records no run,
// unless a stored subject was masked under another scanner than the one the pass masks with, which
// reopens it, and a pass reopened after its enumeration ended only fetches the stale subjects again
// (ADR-0120).
func Open(ctx context.Context, deps Deps, account string) (*Pass, error) {
	stamp := index.StampOf(deps.Scanner)
	ended, stale, latest, err := deps.Store.State(ctx, account, stamp)
	if err != nil {
		return nil, fmt.Errorf("reading the account's first pass: %w", err)
	}
	start, over := core.Under(core.Begin(ended, stale > 0, latest), latest, stamp, stale)
	if start.Skip {
		return &Pass{deps: deps, account: account, done: true}, nil
	}
	p := &Pass{deps: deps, account: account, run: deps.RunID(), at: start.From}
	if start.ResumedFrom != "" && !over {
		p.resumed = start.From.Checkpoint.Token
	}
	if err := deps.Store.Start(ctx, account, p.run, start); err != nil {
		return nil, fmt.Errorf("recording the run: %w", err)
	}
	if over {
		e := Event{Kind: "retry", Detail: "the run it resumes enumerated under another scanner, so the pass starts over from the first page"}
		if err := deps.Store.Event(ctx, account, p.run, e); err != nil {
			return nil, p.failed(ctx, err)
		}
	}
	return p, nil
}

// Run returns the run's identifier, empty for a pass that opened done.
func (p *Pass) Run() string { return p.run }

// Progress returns the checkpoint and counters the run has made durable.
func (p *Pass) Progress() core.Progress { return p.at }

// Next makes the next page durable, or once the enumeration has ended the next call fetching stale
// subjects again, or finishes the pass once no stored subject is stale. What the run does after a
// failed attempt at a page or a call is core.OnFailure's decision, which Next enacts, recording the
// failed page or messages with what became of them. A cancelled ctx returns at once and records
// nothing more, as a process stopped at that point would. A panic raised in the step is recovered
// and recorded as the run's failure with its stack, as any other failure is (ADR-0119).
func (p *Pass) Next(ctx context.Context) (step Step, err error) {
	if p.done {
		return Step{Done: true}, nil
	}
	defer func() {
		if r := recover(); r != nil {
			step, err = Step{}, p.failed(ctx, schedule.Panicked(r))
		}
	}()
	if p.at.Checkpoint.Ended() {
		return p.refetch(ctx)
	}
	page, recovered, err := p.fetch(ctx)
	if err != nil {
		return Step{}, err
	}
	decided := index.Decide(page.Items, p.deps.Policy(), p.deps.Scanner, p.deps.Lookups)
	from := p.at
	c, err := p.deps.Store.Commit(ctx, p.account, p.run, decided, recovered, func(added, remasked int) core.Progress {
		return core.Advance(from, page.Next, page.Total, added, remasked)
	})
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("making page %d durable: %w", from.Checkpoint.Page+1, err))
	}
	p.at, p.resumed = c.Progress, ""
	made := Step{Unclassified: c.Unclassified}
	if p.at.Checkpoint.Ended() {
		// The page that ended the enumeration ends the pass too, unless a subject is stale, which the
		// next steps fetch again.
		finished, err := p.finish(ctx)
		finished.Unclassified = made.Unclassified
		return finished, err
	}
	return made, nil
}

// refetch fetches again by identifier, in one call, the next stale subjects the index holds, masks
// each again from the subject the provider returns, masks whole the subject of a message the provider
// left out, and makes the call durable. With no stale subject left it finishes the pass (ADR-0120).
func (p *Pass) refetch(ctx context.Context) (Step, error) {
	stamp := p.at.Checkpoint.Stamp
	stored, err := p.deps.Store.Stale(ctx, p.account, stamp, max(p.deps.PerCall, 1))
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("reading the subjects masked under another scanner: %w", err))
	}
	if len(stored) == 0 {
		return p.finish(ctx)
	}
	ids := make([]string, 0, len(stored))
	for _, m := range stored {
		ids = append(ids, m.ID)
	}
	got, recovered, err := p.metadata(ctx, ids)
	if err != nil {
		return Step{}, err
	}
	remasked, gone := core.Refetch(stored, got, p.deps.Scanner)
	from := p.at
	at, err := p.deps.Store.Refetch(ctx, p.account, p.run, stamp, Refetched{Remasked: remasked, Gone: gone, Recovered: recovered, Now: p.deps.Now().UTC()},
		func(fetched, stale int) core.Progress { return core.Fetched(from, fetched, stale) })
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("making the subjects fetched again durable: %w", err))
	}
	// A call moves the pass when it stores a subject under the pair in force, masked or not, or when
	// fewer subjects are left stale. One that moves nothing left every subject it asked for stale, so
	// the next read would ask for the same ones and the pass would spend from the budget without end.
	moved := at.Counters.Refetched > from.Counters.Refetched || at.Checkpoint.Stale < from.Checkpoint.Stale
	p.at = at
	if !moved {
		return Step{}, p.failed(ctx, fmt.Errorf("fetching %d subjects again left every one of them stale", len(stored)))
	}
	return Step{}, nil
}

// finish records that the run ended the pass, once no stored subject is stale. A subject made stale
// since the last read leaves the pass running, and the next step fetches it (ADR-0120).
func (p *Pass) finish(ctx context.Context) (Step, error) {
	ended, err := p.deps.Store.Finish(ctx, p.account, p.run, p.at.Checkpoint.Stamp)
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("finishing the pass: %w", err))
	}
	if !ended {
		return Step{}, nil
	}
	p.done = true
	return Step{Done: true}, nil
}

// metadata asks for the metadata of ids in one call until it arrives or the run fails on it. It
// returns the answer with an item for each message the call names when earlier attempts at it failed,
// recorded as recovered with the answer.
func (p *Pass) metadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, []Item, error) {
	var failure *Item
	for {
		got, err := p.deps.Metadata(ctx, ids)
		if err == nil {
			if failure == nil {
				return got, nil, nil
			}
			failure.Attempts++
			failure.Last = p.deps.Now().UTC()
			failure.Disposition = "recovered"
			return got, itemsOf(*failure, ids), nil
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		class := index.ClassOf(err)
		now := p.deps.Now().UTC()
		if failure == nil {
			failure = &Item{Kind: "message", First: now}
		}
		failure.Class, failure.Summary, failure.Last = string(class), err.Error(), now
		failure.Attempts++
		var e Event
		switch core.OnFailure(core.Attempt{Class: class, Attempts: failure.Attempts}) {
		case core.FailRun, core.StartOver:
			return nil, nil, p.failed(ctx, fmt.Errorf("fetching %d subjects again: %w", len(ids), err))
		case core.Backoff:
			e = Event{Kind: "backoff", Detail: err.Error()}
		case core.Retry:
			e = Event{Kind: "retry", Detail: err.Error()}
		case core.Abandon:
			failure.Disposition = "abandoned"
			cause := fmt.Errorf("fetching %d subjects again: %w", len(ids), err)
			if recErr := p.deps.Store.Items(ctx, p.account, p.run, itemsOf(*failure, ids)); recErr != nil {
				return nil, nil, p.failed(ctx, errors.Join(cause, recErr))
			}
			return nil, nil, p.failed(ctx, cause)
		}
		if err := p.deps.Store.Event(ctx, p.account, p.run, e); err != nil {
			return nil, nil, p.failed(ctx, err)
		}
	}
}

// itemsOf returns f as an item for each message ids names.
func itemsOf(f Item, ids []string) []Item {
	out := make([]Item, 0, len(ids))
	for _, id := range ids {
		it := f
		it.ID = id
		out = append(out, it)
	}
	return out
}

// fetch asks for the page after the checkpoint until it arrives or the run fails on it. It returns
// the page with the failure its commit records as recovered, when earlier attempts at it failed.
func (p *Pass) fetch(ctx context.Context) (mail.Page[mail.MessageMetadata], *Failure, error) {
	var failure *Failure
	for {
		number, token := p.at.Checkpoint.Page+1, p.at.Checkpoint.Token
		page, err := p.deps.Fetch(ctx, token)
		if err == nil {
			if failure != nil {
				failure.Attempts++
				failure.Last = p.deps.Now().UTC()
				failure.Disposition = "recovered"
			}
			return page, failure, nil
		}
		if ctx.Err() != nil {
			return page, nil, ctx.Err()
		}
		class := index.ClassOf(err)
		now := p.deps.Now().UTC()
		if failure == nil || failure.Page != number {
			failure = &Failure{Page: number, First: now}
		}
		failure.Class, failure.Summary, failure.Last = string(class), err.Error(), now
		failure.Attempts++
		next := core.OnFailure(core.Attempt{
			Class: class, Attempts: failure.Attempts,
			ResumedToken: p.resumed != "" && token == p.resumed, Restarted: p.restarted,
		})
		var e Event
		switch next {
		case core.FailRun:
			return page, nil, p.failed(ctx, fmt.Errorf("asking for page %d: %w", number, err))
		case core.StartOver:
			p.restarted, p.at, failure = true, core.Restart(p.at), nil
			e = Event{Kind: "retry", Page: number, Detail: "the provider refused the page token the run resumed from, so the pass starts over from the first page: " + err.Error()}
		case core.Backoff:
			e = Event{Kind: "backoff", Page: number, Detail: err.Error()}
		case core.Retry:
			e = Event{Kind: "retry", Page: number, Detail: err.Error()}
		case core.Abandon:
			failure.Disposition = "abandoned"
			cause := fmt.Errorf("asking for page %d: %w", number, err)
			if recErr := p.deps.Store.Failure(ctx, p.account, p.run, *failure); recErr != nil {
				return page, nil, p.failed(ctx, errors.Join(cause, recErr))
			}
			return page, nil, p.failed(ctx, cause)
		}
		if err := p.deps.Store.Event(ctx, p.account, p.run, e); err != nil {
			return page, nil, p.failed(ctx, err)
		}
	}
}

// failed records the run as failed with cause, unless ctx was cancelled, and returns cause with any
// error recording it.
func (p *Pass) failed(ctx context.Context, cause error) error {
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), cause)
	}
	p.done = true
	if err := p.deps.Store.Fail(ctx, p.account, p.run, cause.Error()); err != nil {
		return errors.Join(cause, fmt.Errorf("recording the run's failure: %w", err))
	}
	return cause
}
