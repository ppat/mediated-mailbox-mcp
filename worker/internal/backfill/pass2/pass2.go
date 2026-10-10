// Package pass2 runs backfill's second pass over one account, reading the messages waiting for a
// scan a page at a time, deciding each through the scan gate, scanning the bodies it selects, and
// making each page durable with its checkpoint in one transaction (ADR-0017). The pass's decisions
// come from worker/internal/core/backfill/pass2, and this package enacts them.
//
// A Pass is opened once per run and asked for its next page until it is done. Opening it is the
// recovery path. It reads the account's latest run and resumes from its checkpoint, and it runs the
// delisting transition, restricts the stored classes a rule added since restricts, and returns to
// pending each skip decided without its subject's signal before the first page (ADR-0037, ADR-0113,
// ADR-0120). Reopen, which backfill's run-start step calls before the first pass, returns to pending
// every verdict made under another scanner and every stored gate skip the gate no longer decides as
// the same skip (ADR-0120, ADR-0121). A restricted sender's body is never asked for
// (ADR-0008). A body the gate selects is fetched under a lease in the batch class, converted and
// scanned in memory, and dropped, so nothing of it reaches the store or a log (ADR-0009).
package pass2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	pass1core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// Store is where a pass keeps the index and its runs. Each method is one transaction.
type Store interface {
	// State returns whether the account's first and second passes have ended, whether backfill's
	// run-start step marked the second to start over (ADR-0120), and its latest recorded run of the
	// second.
	State(ctx context.Context, account string) (firstEnded, ended, restart bool, latest pass1core.Latest[core.Progress], err error)
	// Start records the run runID as it starts from start, recording the run it resumes as failed
	// first when start abandons it and clearing the mark that starts the pass over, with a start
	// event, or a resume event naming the checkpoint page.
	Start(ctx context.Context, account, runID string, start pass1core.Start[core.Progress]) error
	// Delist runs the delisting transition. It reads the domains of the messages stored as restricted
	// or skipped as restricted, returns those messages of every domain delisted names to a normal
	// sender class and pending scan with its sender's statistics rebuilt, and, when it marked any
	// message, records the run's progress as restart with an event, all in one transaction. It returns
	// how many messages it marked.
	Delist(ctx context.Context, account, runID string, delisted func(restricted []string) []string, restart core.Progress) (int, error)
	// List restricts the stored classes a rule added since restricts. It reads the domains of the
	// messages stored as normal, sets those messages of every domain listed names to the restricted
	// class with the rule listed gives it, and rebuilds its sender's statistics, all in one
	// transaction. It leaves every scan state as it is, so the run's progress stands. It returns how
	// many messages it marked (ADR-0113).
	List(ctx context.Context, account string, listed func(normal []string) []index.Listing) (int, error)
	// Reopen returns to pending scan each scanned message whose verdict was made under another scanner
	// than the one stamped s, its verdict cleared, and counts again the prior hits of their senders.
	// It then reads, in batches, every subject stored unmasked and masked under another scanner, and
	// stores the subjects remasked returns for them under s with a masking event for each mask, a batch
	// in a few statements. It then reads every message the gate skipped with its gate inputs and
	// returns to pending scan each one overturned names. When it marked any message, masked any
	// subject again, or left a stored subject masked under another scanner so the first pass is due
	// again, it records the pass as not ended and marks it to start over. It writes to no run's record.
	// It is one transaction, which backfill's run-start step makes before the first pass (ADR-0120,
	// ADR-0121). It returns how many of each it marked or masked again.
	Reopen(ctx context.Context, account string, s index.Stamp, remasked func(stored []pass1core.Stored) []index.Message, overturned func(skips []index.Waiting) []string) (Reopened, error)
	// RequeueSkips returns to pending scan each message the gate skipped whose subject is masked, with
	// an event when it marked any, in one transaction (ADR-0120). It returns how many it marked.
	RequeueSkips(ctx context.Context, account, runID string) (int, error)
	// Pending returns up to n of the account's messages waiting for a scan after the message
	// identified by after, in the order of their identifiers.
	Pending(ctx context.Context, account, after string, n int) ([]index.Waiting, error)
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
	// Policy returns the account's policy in the active snapshot, which each page takes at its entry,
	// so a policy edit reaches the pages after it (ADR-0041, ADR-0119). Lookups are the classifier's
	// domain functions, Gate the scan gate's thresholds and Scanner the scanner bodies are scanned
	// with.
	Policy  func() policy.Composed
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
	// compared is the policy the run last made the delisting and added-rule comparisons under.
	compared policy.Composed
}

// Step is what one call of Next did.
type Step struct {
	// Done is set once the pass has ended for the account, or was found ended or not yet due when
	// the run opened.
	Done bool
}

// Reopened counts what backfill's run-start step returned to pending, the verdicts made under another
// scanner and the gate skips the gate no longer decides as the same skip, and the subjects stored
// unmasked it masked again from the store.
type Reopened struct {
	Verdicts, Skips, Subjects int
}

// Reopen returns to pending every verdict made under another scanner than the one the pass scans
// with, masks again from the store, with that scanner, every subject stored unmasked under another,
// and then returns to pending every stored gate skip the gate, deciding it again under the pass's
// thresholds and policy with the message's age measured at the pass's clock, no longer decides as the
// same skip, so those decisions see the subjects as now masked. It reopens the pass when it did any of
// these or when the first pass is due again, so a stale verdict or an overturned skip is denied from the
// start of the run that sees it, whatever the first pass's state. Backfill's run-start step calls it
// before the first pass (ADR-0120, ADR-0121).
func Reopen(ctx context.Context, deps Deps, account string) (Reopened, error) {
	now := mail.UnixMilli(deps.Now().UnixMilli())
	composed := deps.Policy()
	r, err := deps.Store.Reopen(ctx, account, index.StampOf(deps.Scanner), func(stored []pass1core.Stored) []index.Message {
		return pass1core.Remasked(stored, deps.Scanner)
	}, func(skips []index.Waiting) []string {
		return core.Overturned(composed, deps.Lookups, deps.Gate, now, skips)
	})
	if err != nil {
		return Reopened{}, fmt.Errorf("returning the verdicts another scanner made and the overturned gate skips to pending, and masking the subjects stored unmasked again: %w", err)
	}
	return r, nil
}

// Open starts a run of the pass over the account, resuming its latest run when that run stopped
// before the pass ended, runs the delisting transition, restricts the stored classes a rule added
// since restricts, and returns to pending each skip decided without its subject's signal. A pass that has ended, or whose first pass has not, opens done and
// records no run. Reopen is what records the pass as not ended after a change of scanner or of what
// the gate decides (ADR-0120, ADR-0121).
func Open(ctx context.Context, deps Deps, account string) (pass *Pass, err error) {
	firstEnded, ended, startOver, latest, err := deps.Store.State(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("reading the account's second pass: %w", err)
	}
	start, over := core.Over(core.Begin(firstEnded, ended, latest), startOver)
	if start.Skip {
		return &Pass{deps: deps, account: account, done: true}, nil
	}
	p := &Pass{deps: deps, account: account, run: deps.RunID(), at: start.From}
	if err := deps.Store.Start(ctx, account, p.run, start); err != nil {
		return nil, fmt.Errorf("recording the run: %w", err)
	}
	// A panic in the comparisons the run starts with is recovered here, once the run is recorded, so it
	// is recorded as the run's failure with its stack, as a panic in a page is (ADR-0119).
	defer func() {
		if r := recover(); r != nil {
			pass, err = nil, p.failed(ctx, schedule.Panicked(r))
		}
	}()
	if over {
		e := pass1.Event{Kind: "retry", Detail: "a backfill run returned work to pending since the run it resumes stopped, so the pass starts over from the first message waiting for a scan"}
		if err := deps.Store.Event(ctx, account, p.run, e); err != nil {
			return nil, p.failed(ctx, err)
		}
	}
	if err := p.compare(ctx, deps.Policy()); err != nil {
		return nil, err
	}
	// A skip is returned to pending only after a re-mask, in a run Reopen already started from the first
	// waiting message, so returning it needs no restart of its own.
	if _, err := deps.Store.RequeueSkips(ctx, account, p.run); err != nil {
		return nil, p.failed(ctx, fmt.Errorf("returning to pending the skips decided without their subject's signal: %w", err))
	}
	return p, nil
}

// compare makes the delisting comparison and the added-rule comparison under composed, and starts the
// run over from the first waiting message when the delisting marked any message (ADR-0037, ADR-0113).
// The run makes them at its start, and again at the first page after the policy it holds differs by
// value from the active one, so a policy edit reaches the stored classes within one reload and one
// page (ADR-0119).
func (p *Pass) compare(ctx context.Context, composed policy.Composed) error {
	restart := core.Restart(p.at)
	marked, err := p.deps.Store.Delist(ctx, p.account, p.run, func(restricted []string) []string {
		return index.Delisted(composed, p.deps.Lookups, restricted)
	}, restart)
	if err != nil {
		return p.failed(ctx, fmt.Errorf("running the delisting transition: %w", err))
	}
	if _, err := p.deps.Store.List(ctx, p.account, func(normal []string) []index.Listing {
		return index.Listed(composed, p.deps.Lookups, normal)
	}); err != nil {
		return p.failed(ctx, fmt.Errorf("restricting the stored classes a rule added since restricts: %w", err))
	}
	if marked > 0 {
		p.at = restart
	}
	p.compared = composed
	return nil
}

// Run returns the run's identifier, empty for a pass that opened done.
func (p *Pass) Run() string { return p.run }

// Progress returns the checkpoint and counters the run has made durable.
func (p *Pass) Progress() core.Progress { return p.at }

// Next makes the next page durable, or finishes the pass once no message waits after the
// checkpoint. Each message is decided by the gate in turn, after the scan of the one before it, so a
// hit reaches the sender's next message on the same page. What the run does after a failed attempt at
// a body is core.OnFailure's decision, which Next enacts. A cancelled ctx returns at once and records
// nothing more, as a process stopped at that point would. The page takes the active policy at its
// entry, and when it differs by value from the one the run last compared under, the page first makes
// the delisting and added-rule comparisons again (ADR-0119). A panic raised in the page is recovered
// and recorded as the run's failure with its stack, as any other failure is.
func (p *Pass) Next(ctx context.Context) (step Step, err error) {
	if p.done {
		return Step{Done: true}, nil
	}
	defer func() {
		if r := recover(); r != nil {
			step, err = Step{}, p.failed(ctx, schedule.Panicked(r))
		}
	}()
	composed := p.deps.Policy()
	if !composed.Equal(p.compared) {
		if err := p.compare(ctx, composed); err != nil {
			return Step{}, err
		}
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
		o := index.Outcome{ID: m.ID, Domain: m.Domain, Verdict: index.Gate(composed, p.deps.Lookups, p.deps.Gate, now, m, hits[m.Domain])}
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
func (p *Pass) scan(ctx context.Context, id string, number int) (*index.Scanned, *pass1.Item, error) {
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
				Kind: "message", ID: id, Page: number, Class: string(index.Validation),
				Summary: "the conversion refused the body: " + cerr.Error(), Attempts: 1, First: now, Last: now, Disposition: "abandoned",
			}
			if item != nil {
				refused.Attempts, refused.First = item.Attempts, item.First
			}
			return nil, refused, nil
		}
		md = converted
	}
	scanned := index.Scan(p.deps.Scanner, md, body.Text)
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
		class := index.ClassOf(err)
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
