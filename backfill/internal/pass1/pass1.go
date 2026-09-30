// Package pass1 runs backfill's first pass over one account, enumerating every page of the mailbox's
// metadata and making each page durable with its checkpoint in one transaction (ADR-0017). The pass's
// decisions come from backfill/internal/core/pass1, and this package enacts them.
//
// A Pass is opened once per run and asked for its next page until it is done. Opening it is the
// recovery path. It reads the account's latest run and resumes from its checkpoint, so a run killed
// at any point repeats at most the one page that was not yet durable. Everything a page adds reaches
// the Store in one call, which makes the page's rows, its masking events, its senders' statistics and
// the run's checkpoint durable together or not at all.
package pass1

import (
	"context"
	"errors"
	"fmt"
	"time"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
)

// Event is one event on a run's timeline, beside the start, progress, finish and failure events the
// Store records with the change they mark (ADR-0022).
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

// Committed is what a page's commit made durable.
type Committed struct {
	Progress core.Progress
	// Unclassified counts the messages the commit added whose sender could not be classified, and
	// leaves out the ones the index already held.
	Unclassified int
}

// Store is where a pass keeps the index and its runs. Each method is one transaction.
type Store interface {
	// State returns whether the account's pass has ended and its latest recorded run of the pass.
	State(ctx context.Context, account string) (ended bool, latest core.Latest[core.Progress], err error)
	// Start records the run runID as it starts from start, recording the run it resumes as failed
	// first when start abandons it, with a start event, or a resume event naming the checkpoint page.
	Start(ctx context.Context, account, runID string, start core.Start[core.Progress]) error
	// Commit makes one page durable. It adds each of the page's messages the index does not hold yet,
	// records a masking event for each mask on the subject of a message it added and for no other,
	// rebuilds the statistics of the page's senders, records recovered when earlier attempts at the
	// page failed, and records the progress advance returns for the number of messages it added,
	// with a progress event, all in one transaction.
	Commit(ctx context.Context, account, runID string, page core.Page, recovered *Failure, advance func(added int) core.Progress) (Committed, error)
	// Finish records that the run ended the pass. It sets the account's completion flag and records
	// the run as succeeded with a finish event.
	Finish(ctx context.Context, account, runID string) error
	// Fail records the run as failed with its last error, and a failure event.
	Fail(ctx context.Context, account, runID, cause string) error
	// Event adds an event to the run's timeline.
	Event(ctx context.Context, account, runID string, e Event) error
	// Failure records one page the run failed on.
	Failure(ctx context.Context, account, runID string, f Failure) error
}

// Fetch returns the page of the account's enumeration that token names, the empty token naming the
// first. Its errors wrap the Provider Port's.
type Fetch func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error)

// Deps are what a Pass runs with.
type Deps struct {
	Store Store
	Fetch Fetch
	// Policy is the account's policy, Scanner the scanner subjects are masked with, and Lookups the
	// classifier's domain functions.
	Policy  policy.Composed
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
	// Done is set once the pass has ended for the account, or was found ended when the run opened.
	Done bool
	// Unclassified counts the messages of the page made durable whose sender could not be classified.
	Unclassified int
}

// Open starts a run of the pass over the account, resuming its latest run when that run stopped
// before the pass ended. A pass that has ended opens done, and records no run.
func Open(ctx context.Context, deps Deps, account string) (*Pass, error) {
	ended, latest, err := deps.Store.State(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("reading the account's first pass: %w", err)
	}
	start := core.Begin(ended, latest)
	if start.Skip {
		return &Pass{deps: deps, account: account, done: true}, nil
	}
	p := &Pass{deps: deps, account: account, run: deps.RunID(), at: start.From}
	if start.ResumedFrom != "" {
		p.resumed = start.From.Checkpoint.Token
	}
	if err := deps.Store.Start(ctx, account, p.run, start); err != nil {
		return nil, fmt.Errorf("recording the run: %w", err)
	}
	return p, nil
}

// Run returns the run's identifier, empty for a pass that opened done.
func (p *Pass) Run() string { return p.run }

// Progress returns the checkpoint and counters the run has made durable.
func (p *Pass) Progress() core.Progress { return p.at }

// Next makes the next page durable, or finishes the pass once the enumeration has ended. What the run
// does after a failed attempt at a page is core.OnFailure's decision, which Next enacts, recording a
// failed page with what became of it. A cancelled ctx returns at once and records nothing more, as a
// process stopped at that point would.
func (p *Pass) Next(ctx context.Context) (Step, error) {
	if p.done {
		return Step{Done: true}, nil
	}
	if p.at.Checkpoint.Ended() {
		return p.finish(ctx)
	}
	page, recovered, err := p.fetch(ctx)
	if err != nil {
		return Step{}, err
	}
	decided := core.Decide(page.Items, p.deps.Policy, p.deps.Scanner, p.deps.Lookups)
	from := p.at
	c, err := p.deps.Store.Commit(ctx, p.account, p.run, decided, recovered, func(added int) core.Progress {
		return core.Advance(from, page.Next, added)
	})
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("making page %d durable: %w", from.Checkpoint.Page+1, err))
	}
	p.at, p.resumed = c.Progress, ""
	step := Step{Unclassified: c.Unclassified}
	if p.at.Checkpoint.Ended() {
		finished, err := p.finish(ctx)
		finished.Unclassified = step.Unclassified
		return finished, err
	}
	return step, nil
}

// finish records that the run ended the pass.
func (p *Pass) finish(ctx context.Context) (Step, error) {
	if err := p.deps.Store.Finish(ctx, p.account, p.run); err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("finishing the pass: %w", err))
	}
	p.done = true
	return Step{Done: true}, nil
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
		class := core.ClassOf(err)
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
