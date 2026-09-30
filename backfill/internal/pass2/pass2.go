// Package pass2 runs backfill's second pass over one account, reading the messages waiting for a
// scan a page at a time, deciding each through the scan gate, scanning the bodies it selects, and
// making each page durable with its checkpoint in one transaction (ADR-0017). The pass's decisions
// come from backfill/internal/core/pass2, and this package enacts them.
//
// A Pass is opened once per run and asked for its next page until it is done. Opening it is the
// recovery path. It reads the account's latest run and resumes from its checkpoint, and it runs the
// delisting transition before the first page (ADR-0037). A restricted sender's body is never asked
// for (ADR-0008). A body the gate selects is fetched under a lease in the batch class, converted and
// scanned in memory, and dropped, so nothing of it reaches the store or a log (ADR-0009).
package pass2

import (
	"context"
	"errors"
	"fmt"
	"time"

	pass1core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/sanitize/markdown"
)

// Store is where a pass keeps the index and its runs. Each method is one transaction.
type Store interface {
	// State returns whether the account's first and second passes have ended, and its latest
	// recorded run of the second.
	State(ctx context.Context, account string) (firstEnded, ended bool, latest pass1core.Latest[core.Progress], err error)
	// Start records the run runID as it starts from start, recording the run it resumes as failed
	// first when start abandons it, with a start event, or a resume event naming the checkpoint page.
	Start(ctx context.Context, account, runID string, start pass1core.Start[core.Progress]) error
	// Delist runs the delisting transition. It reads the domains of the messages stored as restricted
	// or skipped as restricted, returns those messages of every domain delisted names to a normal
	// sender class and pending scan with its sender's statistics rebuilt, and, when it marked any
	// message, records the run's progress as restart with an event, all in one transaction. It returns
	// how many messages it marked.
	Delist(ctx context.Context, account, runID string, delisted func(restricted []string) []string, restart core.Progress) (int, error)
	// Pending returns up to n of the account's messages waiting for a scan after the message
	// identified by after, in the order of their identifiers.
	Pending(ctx context.Context, account, after string, n int) ([]core.Message, error)
	// Commit makes one page durable. It records the gate's decision on each message the gate decided,
	// the verdict of each message scanned and the state of each skipped, adds the page's hits to its
	// senders' prior hits, records the page's failed items, and records the progress at with a
	// progress event, all in one transaction.
	Commit(ctx context.Context, account, runID string, page core.Page, items []pass1.Item, at core.Progress) error
	// Finish records that the run ended the pass. It sets the account's completion flag and records
	// the run as succeeded with a finish event.
	Finish(ctx context.Context, account, runID string) error
	// Fail records the run as failed with its last error, and a failure event.
	Fail(ctx context.Context, account, runID, cause string) error
	// Event adds an event to the run's timeline.
	Event(ctx context.Context, account, runID string, e pass1.Event) error
	// Failure records one item the run failed on.
	Failure(ctx context.Context, account, runID string, it pass1.Item) error
	// Backlog returns how many of the account's messages wait for a scan.
	Backlog(ctx context.Context, account string) (int64, error)
}

// Body returns a message's body. Its errors wrap the Provider Port's.
type Body func(ctx context.Context, id string) (mail.MessageBody, error)

// Deps are what a Pass runs with.
type Deps struct {
	Store Store
	Body  Body
	// Policy is the account's policy, Lookups the classifier's domain functions, Gate the scan gate's
	// thresholds and Scanner the scanner bodies are scanned with.
	Policy  policy.Composed
	Lookups classify.Lookups
	Gate    scangate.Config
	Scanner scan.Scanner
	// PageSize is how many waiting messages one page reads.
	PageSize int
	// RunID returns a new run's identifier, a short opaque string.
	RunID func() string
	// Now is the clock a message's age and a failure's attempts are read from.
	Now func() time.Time
}

// Pass is one run of the second pass over one account.
type Pass struct {
	deps    Deps
	account string
	run     string
	at      core.Progress
	done    bool
}

// Step is what one call of Next did.
type Step struct {
	// Done is set once the pass has ended for the account, or was found ended or not yet due when
	// the run opened.
	Done bool
}

// Open starts a run of the pass over the account, resuming its latest run when that run stopped
// before the pass ended, and runs the delisting transition. A pass that has ended, or whose first pass
// has not, opens done and records no run.
func Open(ctx context.Context, deps Deps, account string) (*Pass, error) {
	firstEnded, ended, latest, err := deps.Store.State(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("reading the account's second pass: %w", err)
	}
	start := core.Begin(firstEnded, ended, latest)
	if start.Skip {
		return &Pass{deps: deps, account: account, done: true}, nil
	}
	p := &Pass{deps: deps, account: account, run: deps.RunID(), at: start.From}
	if err := deps.Store.Start(ctx, account, p.run, start); err != nil {
		return nil, fmt.Errorf("recording the run: %w", err)
	}
	restart := core.Restart(p.at)
	marked, err := deps.Store.Delist(ctx, account, p.run, func(restricted []string) []string {
		return core.Delisted(deps.Policy, deps.Lookups, restricted)
	}, restart)
	if err != nil {
		return nil, p.failed(ctx, fmt.Errorf("running the delisting transition: %w", err))
	}
	if marked > 0 {
		p.at = restart
	}
	return p, nil
}

// Run returns the run's identifier, empty for a pass that opened done.
func (p *Pass) Run() string { return p.run }

// Progress returns the checkpoint and counters the run has made durable.
func (p *Pass) Progress() core.Progress { return p.at }

// Next makes the next page durable, or finishes the pass once no message waits after the
// checkpoint. Each message is decided by the gate in turn, after the scan of the one before it, so a
// hit reaches the sender's next message on the same page. What the run does after a failed attempt at
// a body is core.OnFailure's decision, which Next enacts. A cancelled ctx returns at once and records
// nothing more, as a process stopped at that point would.
func (p *Pass) Next(ctx context.Context) (Step, error) {
	if p.done {
		return Step{Done: true}, nil
	}
	messages, err := p.deps.Store.Pending(ctx, p.account, p.at.Checkpoint.After, p.deps.PageSize)
	if err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("reading the messages waiting for a scan: %w", err))
	}
	if len(messages) == 0 {
		return p.finish(ctx)
	}
	number := p.at.Checkpoint.Page + 1
	now := mail.UnixMilli(p.deps.Now().UnixMilli())
	page := core.Page{Last: messages[len(messages)-1].ID}
	hits := map[string]int64{}
	var items []pass1.Item
	for _, m := range messages {
		o := core.Outcome{ID: m.ID, Domain: m.Domain, Verdict: core.Gate(p.deps.Policy, p.deps.Lookups, p.deps.Gate, now, m, hits[m.Domain])}
		if o.Verdict.Scans() {
			scanned, item, err := p.scan(ctx, m.ID, number)
			if err != nil {
				return Step{}, err
			}
			if item != nil {
				items = append(items, *item)
			}
			if scanned != nil {
				o.Scanned = scanned
				if scanned.Flags.Any() {
					hits[m.Domain]++
				}
			}
		}
		page.Outcomes = append(page.Outcomes, o)
	}
	at := core.Advance(p.at, page)
	if err := p.deps.Store.Commit(ctx, p.account, p.run, page, items, at); err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("making page %d durable: %w", number, err))
	}
	p.at = at
	return Step{}, nil
}

// scan fetches the message's body, converts its HTML part and scans both parts. It returns what the
// scan records, or the failed item that leaves the message waiting, or with neither the error that
// ends the run. The body is dropped when it returns.
func (p *Pass) scan(ctx context.Context, id string, number int) (*core.Scanned, *pass1.Item, error) {
	body, item, err := p.fetch(ctx, id, number)
	if err != nil || item != nil && item.Disposition != "recovered" {
		return nil, item, err
	}
	var md string
	if body.HTML != "" {
		converted, cerr := markdown.Convert(body.HTML)
		if cerr != nil {
			now := p.deps.Now().UTC()
			refused := &pass1.Item{
				Kind: "message", ID: id, Page: number, Class: string(pass1core.Validation),
				Summary: "the conversion refused the body: " + cerr.Error(), Attempts: 1, First: now, Last: now, Disposition: "abandoned",
			}
			if item != nil {
				refused.Attempts, refused.First = item.Attempts, item.First
			}
			return nil, refused, nil
		}
		md = converted
	}
	scanned := core.Scan(p.deps.Scanner, md, body.Text)
	return &scanned, item, nil
}

// fetch asks for the message's body until it arrives or the run gives up on it. It returns the body
// with the item its commit records as recovered, when earlier attempts failed, or the item that
// leaves the message waiting, or the error that ends the run.
func (p *Pass) fetch(ctx context.Context, id string, number int) (mail.MessageBody, *pass1.Item, error) {
	var item *pass1.Item
	for {
		body, err := p.deps.Body(ctx, id)
		if err == nil {
			if item != nil {
				item.Attempts++
				item.Last = p.deps.Now().UTC()
				item.Disposition = "recovered"
			}
			return body, item, nil
		}
		if ctx.Err() != nil {
			return mail.MessageBody{}, nil, ctx.Err()
		}
		class := pass1core.ClassOf(err)
		now := p.deps.Now().UTC()
		if item == nil {
			item = &pass1.Item{Kind: "message", ID: id, Page: number, First: now}
		}
		item.Class, item.Summary, item.Last = string(class), err.Error(), now
		item.Attempts++
		var e pass1.Event
		switch core.OnFailure(core.Attempt{Class: class, Attempts: item.Attempts}) {
		case core.FailRun:
			return mail.MessageBody{}, nil, p.failed(ctx, fmt.Errorf("asking for the body of message %s: %w", id, err))
		case core.Gone:
			item.Disposition = "gone"
			return mail.MessageBody{}, item, nil
		case core.Abandon:
			item.Disposition = "abandoned"
			return mail.MessageBody{}, item, nil
		case core.Backoff:
			e = pass1.Event{Kind: "backoff", Page: number, Detail: err.Error()}
		case core.Retry:
			e = pass1.Event{Kind: "retry", Page: number, Detail: err.Error()}
		case core.Stop:
			item.Disposition = "abandoned"
			cause := fmt.Errorf("asking for the body of message %s: %w", id, err)
			if recErr := p.deps.Store.Failure(ctx, p.account, p.run, *item); recErr != nil {
				return mail.MessageBody{}, nil, p.failed(ctx, errors.Join(cause, recErr))
			}
			return mail.MessageBody{}, nil, p.failed(ctx, cause)
		}
		if err := p.deps.Store.Event(ctx, p.account, p.run, e); err != nil {
			return mail.MessageBody{}, nil, p.failed(ctx, err)
		}
	}
}

// finish records that the run ended the pass.
func (p *Pass) finish(ctx context.Context) (Step, error) {
	if err := p.deps.Store.Finish(ctx, p.account, p.run); err != nil {
		return Step{}, p.failed(ctx, fmt.Errorf("finishing the pass: %w", err))
	}
	p.done = true
	return Step{Done: true}, nil
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
