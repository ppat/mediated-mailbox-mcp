// Package tick runs one delta sync tick over one account (ADR-0018). A tick asks the provider for the
// changes since the account's cursor, applies each change set to the index and advances the cursor in
// the transaction that makes the change set durable, so the cursor never runs ahead of the index. A
// cursor gap starts a recovery recorded as a run of its own, and an account with no cursor is
// reconciled over the first window (ADR-0105). Once backfill's second pass has ended for the account,
// the tick then runs the delisting comparison and its counterpart for an added rule, and decides and
// scans a bounded number of the messages waiting for a scan (ADR-0104, ADR-0113).
//
// What a message adds to the index, the two comparisons, the gate and the scan are core/index's,
// the decisions backfill makes the same way, and the tick's own decisions are
// worker/internal/core/deltasync/tick's. A body is fetched, converted and scanned in memory and dropped, so
// nothing of it reaches the store or a log (ADR-0009).
package tick

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
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/tick"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// The passes job_runs names a tick and a gap's recovery (ADR-0016).
const (
	PassTick        = "tick"
	PassGapRecovery = "gap_recovery"
)

// Deps are what a tick runs with.
type Deps struct {
	Store    Store
	Provider Provider
	// Policy is the account's policy, Lookups the classifier's domain functions, Gate the scan gate's
	// thresholds and Scanner the scanner subjects are masked and bodies scanned with, the same as
	// backfill's (ADR-0120, ADR-0121).
	Policy  policy.Composed
	Lookups classify.Lookups
	Gate    scangate.Config
	Scanner scan.Scanner
	// FirstWindow is how far back an account with no cursor is reconciled (ADR-0105).
	FirstWindow time.Duration
	// Decisions bounds how many waiting messages one tick decides, and PageSize how many one read
	// returns (ADR-0104).
	Decisions, PageSize int
	// RunID returns a new run's identifier, a short opaque string.
	RunID func() string
	// Now is the clock a message's age, a window and a failure's attempts are read from.
	Now func() time.Time
}

// Result is what a tick did.
type Result struct {
	// Run is the tick's run, and Recovery the run of the gap's recovery it started, empty for none.
	Run, Recovery string
	// Gap is set when the tick found a cursor gap.
	Gap bool
	// Unclassified counts the messages the tick added whose sender could not be classified.
	Unclassified int
	// Scanning is set when backfill's second pass has ended, so the tick scanned, and Backlog is then
	// how many of the account's messages wait for a scan once it is done.
	Scanning bool
	Backlog  int64
	// Progress is the tick's checkpoint and counters as it recorded them last.
	Progress Progress
}

// Run runs one tick over the account. A failure is recorded on the tick's run and returned. A
// cancelled ctx returns at once and records nothing more, as a process stopped at that point would.
// A panic raised once the tick's run is recorded is recovered and recorded as the run's failure with
// its stack, as any other failure is (ADR-0119).
func Run(ctx context.Context, deps Deps, account string) (result Result, err error) {
	state, err := deps.Store.State(ctx, account)
	if err != nil {
		return Result{}, fmt.Errorf("reading the account's sync state: %w", err)
	}
	t := &ticker{deps: deps, account: account, run: deps.RunID()}
	t.at.Checkpoint.After = state.ScanAfter
	t.result.Run = t.run
	if err := deps.Store.Start(ctx, account, t.run, PassTick, t.at); err != nil {
		return Result{}, fmt.Errorf("recording the tick: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			result, err = t.result, t.failed(ctx, schedule.Panicked(r))
		}
	}()
	cursor, err := t.changes(ctx, state)
	if err != nil {
		return t.result, t.failed(ctx, err)
	}
	if state.SecondEnded {
		t.result.Scanning = true
		if err := t.scanning(ctx); err != nil {
			return t.result, t.failed(ctx, err)
		}
		backlog, err := deps.Store.Backlog(ctx, account)
		if err != nil {
			return t.result, t.failed(ctx, fmt.Errorf("reading the scan backlog: %w", err))
		}
		t.result.Backlog = backlog
	}
	if err := deps.Store.Finish(ctx, account, t.run, cursor, t.at); err != nil {
		return t.result, t.failed(ctx, fmt.Errorf("finishing the tick: %w", err))
	}
	t.result.Progress = t.at
	return t.result, nil
}

// ticker is one tick over one account.
type ticker struct {
	deps    Deps
	account string
	run     string
	at      Progress
	result  Result
}

// changes applies every change set since the account's cursor, each in the transaction that advances
// the cursor past it. An account with no cursor is reconciled over the first window, and a gap is
// recovered from by a run of its own. It returns the cursor the tick's finish stores, empty when the
// cursor is already stored.
func (t *ticker) changes(ctx context.Context, state State) (mail.Cursor, error) {
	if state.Cursor == "" {
		return t.first(ctx)
	}
	cursor := state.Cursor
	for {
		cs, err := t.deps.Provider.ChangesSince(ctx, cursor)
		if errors.Is(err, mail.ErrCursorGap) {
			t.result.Gap = true
			return "", t.recover(ctx, state)
		}
		if err != nil {
			return "", fmt.Errorf("asking for the changes since the cursor: %w", err)
		}
		empty := len(cs.Added) == 0 && len(cs.Modified) == 0 && len(cs.Removed) == 0
		if empty && cs.Next == cursor {
			return "", nil
		}
		metadata, err := t.deps.Provider.GetMessageMetadata(ctx, core.Fetched(cs))
		if err != nil {
			return "", fmt.Errorf("asking for the metadata of the changed messages: %w", err)
		}
		page := index.Decide(metadata, t.deps.Policy, t.deps.Scanner, t.deps.Lookups)
		next := t.at
		next.Counters.Added += len(cs.Added)
		next.Counters.Modified += len(cs.Modified)
		next.Counters.Removed += len(cs.Removed)
		applied, err := t.deps.Store.Apply(ctx, t.account, t.run, Application{Page: page, Removed: cs.Removed, Cursor: cs.Next}, func(Applied) Recorder { return next })
		if err != nil {
			return "", fmt.Errorf("applying a change set: %w", err)
		}
		t.at = next
		t.result.Unclassified += applied.Unclassified
		if empty || cs.Next == cursor {
			return "", nil
		}
		cursor = cs.Next
	}
}

// first reconciles an account with no cursor over the first window, inside the tick. It takes the
// current cursor before it lists, so nothing that arrives while it lists is missed, and returns it
// for the tick's finish to store (ADR-0105).
func (t *ticker) first(ctx context.Context) (mail.Cursor, error) {
	cursor, err := t.deps.Provider.CurrentCursor(ctx)
	if err != nil {
		return "", fmt.Errorf("asking for the current cursor: %w", err)
	}
	now := mail.UnixMilli(t.deps.Now().UnixMilli())
	start, end := core.Window(nil, now, mail.UnixMilli(t.deps.FirstWindow.Milliseconds()))
	t.at.Counters.Window = window(start, end)
	reconciled, _, err := t.reconcile(ctx, t.run, start, func(n int) Recorder {
		next := t.at
		w := *t.at.Counters.Window
		w.Reconciled = n
		next.Counters.Window = &w
		return next
	})
	if err != nil {
		return "", err
	}
	t.at.Counters.Window.Reconciled = reconciled
	return cursor, nil
}

// recover recovers from a cursor gap in a run of its own. It takes the current cursor first, lists
// the window from an hour before the last cursor's write time to now, applies what it finds, removes
// what the provider no longer holds of the stored messages dated in the window, and stores the cursor
// with the run's success in one transaction. A recovery stopped before then stores nothing of its
// cursor, so the next tick meets the same gap (ADR-0105).
func (t *ticker) recover(ctx context.Context, state State) error {
	run := t.deps.RunID()
	t.result.Recovery = run
	if err := t.deps.Store.Event(ctx, t.account, t.run, Event{Kind: "retry", Detail: "the provider cannot calculate changes from the cursor, so run " + run + " recovers from the gap"}); err != nil {
		return err
	}
	var at Recovery
	if err := t.deps.Store.Start(ctx, t.account, run, PassGapRecovery, at); err != nil {
		return fmt.Errorf("recording the recovery: %w", err)
	}
	fail := func(cause error) error {
		if ctx.Err() != nil {
			return errors.Join(ctx.Err(), cause)
		}
		if err := t.deps.Store.Fail(ctx, t.account, run, cause.Error()); err != nil {
			return errors.Join(cause, fmt.Errorf("recording the recovery's failure: %w", err))
		}
		return fmt.Errorf("recovering from a cursor gap: %w", cause)
	}
	cursor, err := t.deps.Provider.CurrentCursor(ctx)
	if err != nil {
		return fail(fmt.Errorf("asking for the current cursor: %w", err))
	}
	var written *mail.UnixMilli
	if state.CursorAt != nil {
		w := mail.UnixMilli(state.CursorAt.UnixMilli())
		written = &w
	}
	start, end := core.Window(written, mail.UnixMilli(t.deps.Now().UnixMilli()), mail.UnixMilli(t.deps.FirstWindow.Milliseconds()))
	at.Window = *window(start, end)
	progress := func(n int) Recorder {
		next := at
		next.Window.Reconciled = n
		return next
	}
	reconciled, listed, err := t.reconcile(ctx, run, start, progress)
	if err != nil {
		return fail(err)
	}
	removed, err := t.removeGone(ctx, run, start, listed, func(n int) Recorder { return progress(reconciled + n) })
	if err != nil {
		return fail(err)
	}
	at.Window.Reconciled = reconciled + removed
	if err := t.deps.Store.Finish(ctx, t.account, run, cursor, at); err != nil {
		return fail(fmt.Errorf("finishing the recovery: %w", err))
	}
	return nil
}

// reconcile lists the threads holding a message dated at or after start a page at a time, and
// applies every message they hold as a change set's added message is applied, each page in one
// transaction recording the run's progress for the count reconciled so far. It returns how many
// messages it added or whose labels or flags it changed, and every message the whole listing
// returned.
func (t *ticker) reconcile(ctx context.Context, run string, start mail.UnixMilli, progress func(reconciled int) Recorder) (int, map[string]bool, error) {
	reconciled := 0
	listed := map[string]bool{}
	var token mail.PageToken
	for {
		page, err := t.deps.Provider.ListThreads(ctx, mail.After(start), token)
		if err != nil {
			return 0, nil, fmt.Errorf("listing the threads of the window: %w", err)
		}
		var metadata []mail.MessageMetadata
		for _, th := range page.Items {
			metadata = append(metadata, th.Messages...)
			for _, m := range th.Messages {
				listed[m.ID] = true
			}
		}
		decided := index.Decide(metadata, t.deps.Policy, t.deps.Scanner, t.deps.Lookups)
		so := reconciled
		applied, err := t.deps.Store.Apply(ctx, t.account, run, Application{Page: decided}, func(a Applied) Recorder {
			return progress(so + a.Inserted + a.Changed)
		})
		if err != nil {
			return 0, nil, fmt.Errorf("applying the window's messages: %w", err)
		}
		reconciled += applied.Inserted + applied.Changed
		t.result.Unclassified += applied.Unclassified
		if page.Next == "" {
			return reconciled, listed, nil
		}
		token = page.Next
	}
}

// removeGone removes the stored messages dated at or after start that the whole listing did not
// return and that the provider no longer returns when asked for them by their identifiers, in one
// transaction recording the run's progress, and returns how many it removed. A message the listing
// left out but the provider still holds stays (ADR-0105).
func (t *ticker) removeGone(ctx context.Context, run string, start mail.UnixMilli, listed map[string]bool, progress func(removed int) Recorder) (int, error) {
	stored, err := t.deps.Store.Since(ctx, t.account, time.UnixMilli(int64(start)).UTC())
	if err != nil {
		return 0, fmt.Errorf("reading the stored messages of the window: %w", err)
	}
	unlisted := core.Unlisted(stored, listed)
	if len(unlisted) == 0 {
		return 0, nil
	}
	held, err := t.deps.Provider.GetMessageMetadata(ctx, unlisted)
	if err != nil {
		return 0, fmt.Errorf("asking for the stored messages the listing left out: %w", err)
	}
	gone := core.Gone(unlisted, held)
	if len(gone) == 0 {
		return 0, nil
	}
	applied, err := t.deps.Store.Apply(ctx, t.account, run, Application{Removed: gone}, func(a Applied) Recorder { return progress(a.Removed) })
	if err != nil {
		return 0, fmt.Errorf("removing the messages the provider no longer holds: %w", err)
	}
	return applied.Removed, nil
}

// window returns a window's start and end as the counters record them.
func window(start, end mail.UnixMilli) *Window {
	s, e := time.UnixMilli(int64(start)).UTC(), time.UnixMilli(int64(end)).UTC()
	return &Window{Start: s, End: e}
}

// scanning runs the delisting comparison and its counterpart for an added rule, and then decides and
// scans the account's waiting messages from where the last tick stopped, up to the tick's bound.
// Reaching the end of what waits, or marking any message in the delisting comparison, starts the next
// read from the first waiting message (ADR-0037, ADR-0113, ADR-0104).
func (t *ticker) scanning(ctx context.Context) error {
	marked, err := t.deps.Store.Delist(ctx, t.account, t.run, func(restricted []string) []string {
		return index.Delisted(t.deps.Policy, t.deps.Lookups, restricted)
	})
	if err != nil {
		return fmt.Errorf("running the delisting transition: %w", err)
	}
	if marked > 0 {
		t.at.Checkpoint.After = ""
	}
	if _, err := t.deps.Store.List(ctx, t.account, func(normal []string) []index.Listing {
		return index.Listed(t.deps.Policy, t.deps.Lookups, normal)
	}); err != nil {
		return fmt.Errorf("restricting the stored classes a rule added since restricts: %w", err)
	}
	decided := 0
	for decided < t.deps.Decisions {
		waiting, err := t.deps.Store.Pending(ctx, t.account, t.at.Checkpoint.After, min(t.deps.PageSize, t.deps.Decisions-decided))
		if err != nil {
			return fmt.Errorf("reading the messages waiting for a scan: %w", err)
		}
		if len(waiting) == 0 {
			t.at.Checkpoint.After = ""
			return nil
		}
		stop, err := t.page(ctx, waiting)
		if err != nil || stop {
			return err
		}
		decided += len(waiting)
	}
	return nil
}

// page decides and scans one read of waiting messages, each after the scan of the one before it so a
// sender's first hit reaches its next message, and makes it durable in one transaction. It reports
// whether the tick stops scanning, after a throttle or a refused credential. A stop decides nothing
// from the message whose fetch stopped it on, since the gate would decide the rest without the hits
// their unscanned bodies hold, and the checkpoint stays at the last message decided, so the next
// tick reads the stopped one first (ADR-0104).
func (t *ticker) page(ctx context.Context, waiting []index.Waiting) (bool, error) {
	now := t.deps.Now()
	hits := map[string]int64{}
	var outcomes []index.Outcome
	var items []Item
	stop := false
	for _, m := range waiting {
		o := index.Outcome{ID: m.ID, Domain: m.Domain, Verdict: index.Gate(t.deps.Policy, t.deps.Lookups, t.deps.Gate, mail.UnixMilli(now.UnixMilli()), m, hits[m.Domain])}
		if o.Verdict.Scans() {
			scanned, item, next, err := t.scan(ctx, m.ID)
			if err != nil {
				return false, err
			}
			if item != nil {
				items = append(items, *item)
			}
			if next == core.StopScanning {
				stop = true
				break
			}
			if scanned != nil {
				o.Scanned = scanned
				if scanned.Flags.Any() {
					hits[m.Domain]++
				}
			}
		}
		outcomes = append(outcomes, o)
	}
	next := t.at
	if len(outcomes) > 0 {
		next.Checkpoint.After = outcomes[len(outcomes)-1].ID
	}
	for _, o := range outcomes {
		if o.Verdict.Decided() {
			next.Counters.Decided++
		}
		switch {
		case o.Scanned != nil:
			next.Counters.Scanned++
		case o.Verdict.Decided() && !o.Verdict.Scans():
			next.Counters.Skipped++
		default:
			next.Counters.Pending++
		}
	}
	if err := t.deps.Store.CommitScan(ctx, t.account, t.run, outcomes, items, next); err != nil {
		return false, fmt.Errorf("making the scanned messages durable: %w", err)
	}
	t.at = next
	return stop, nil
}

// scan fetches the message's body, converts its HTML part and scans both parts. It returns what the
// scan records, or the failed item that leaves the message waiting with what the tick does next, or
// the error that ends the tick. The body is dropped when it returns.
func (t *ticker) scan(ctx context.Context, id string) (*index.Scanned, *Item, core.Next, error) {
	body, err := t.deps.Provider.GetMessageBody(ctx, id)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, core.FailTick, ctx.Err()
		}
		class := index.ClassOf(err)
		next := core.OnBodyFailure(class)
		if next == core.FailTick {
			return nil, nil, next, fmt.Errorf("asking for the body of message %s: %w", id, err)
		}
		now := t.deps.Now().UTC()
		return nil, &Item{ID: id, Class: string(class), Summary: err.Error(), At: now, Disposition: core.Disposition(class)}, next, nil
	}
	var md string
	if body.HTML != "" {
		converted, err := markdown.Convert(body.HTML)
		if err != nil {
			now := t.deps.Now().UTC()
			return nil, &Item{
				ID: id, Class: string(index.Validation), Summary: "the conversion refused the body: " + err.Error(), At: now, Disposition: "abandoned",
			}, core.LeaveWaiting, nil
		}
		md = converted
	}
	scanned := index.Scan(t.deps.Scanner, md, body.Text)
	return &scanned, nil, core.LeaveWaiting, nil
}

// failed records the tick as failed with cause, unless ctx was cancelled, and returns cause with any
// error recording it.
func (t *ticker) failed(ctx context.Context, cause error) error {
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), cause)
	}
	if err := t.deps.Store.Fail(ctx, t.account, t.run, cause.Error()); err != nil {
		return errors.Join(cause, fmt.Errorf("recording the tick's failure: %w", err))
	}
	return cause
}
