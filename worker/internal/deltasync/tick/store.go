package tick

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// State is what a tick reads of an account before it starts.
type State struct {
	// SecondEnded is set once backfill's second pass has ended for the account (ADR-0104).
	SecondEnded bool
	// Cursor is the stored change cursor, empty before the account's first tick, and CursorAt when it
	// was written, nil when it never was.
	Cursor   mail.Cursor
	CursorAt *time.Time
	// ScanAfter is where the latest tick's scanning stopped, the identifier of the last message it
	// read, empty for the first waiting message.
	ScanAfter string
}

// Recorder is a run's checkpoint and counters in the form job_runs stores them (ADR-0016).
type Recorder interface {
	encode() (checkpoint, counters []byte, err error)
}

// Window is a re-enumeration's window and the messages it reconciled, the ones it added or whose
// labels or flags it changed, and for a gap's recovery the ones it removed (ADR-0105).
type Window struct {
	Start, End time.Time
	Reconciled int
}

// Counters count a tick's work. Added, Modified and Removed count what the change sets reported,
// Window is the first reconciliation of an account with no cursor, and Decided, Scanned, Skipped and
// Pending count the waiting messages the tick read, as backfill's second pass counts them.
type Counters struct {
	Added, Modified, Removed           int
	Window                             *Window
	Decided, Scanned, Skipped, Pending int
}

// Checkpoint is where a tick's scanning stands, the identifier of the last waiting message it read.
type Checkpoint struct {
	After string
}

// Progress is a tick's checkpoint and counters.
type Progress struct {
	Checkpoint Checkpoint
	Counters   Counters
}

// Recovery is a gap recovery's counters, its window and what it reconciled.
type Recovery struct {
	Window Window
}

type windowCounters struct {
	WindowStart *time.Time `json:"window_start,omitempty"`
	WindowEnd   *time.Time `json:"window_end,omitempty"`
	Reconciled  *int       `json:"reconciled,omitempty"`
}

func (w *Window) stored() windowCounters {
	if w == nil {
		return windowCounters{}
	}
	start, end, n := w.Start, w.End, w.Reconciled
	return windowCounters{WindowStart: &start, WindowEnd: &end, Reconciled: &n}
}

func (p Progress) encode() ([]byte, []byte, error) {
	cp, err := json.Marshal(struct {
		After string `json:"after"`
	}{p.Checkpoint.After})
	if err != nil {
		return nil, nil, err
	}
	c := p.Counters
	ct, err := json.Marshal(struct {
		Added    int `json:"added"`
		Modified int `json:"modified"`
		Removed  int `json:"removed"`
		windowCounters
		Decided int `json:"decided"`
		Scanned int `json:"scanned"`
		Skipped int `json:"skipped"`
		Pending int `json:"pending"`
	}{c.Added, c.Modified, c.Removed, c.Window.stored(), c.Decided, c.Scanned, c.Skipped, c.Pending})
	return cp, ct, err
}

func (r Recovery) encode() ([]byte, []byte, error) {
	if r.Window.Start.IsZero() {
		return nil, []byte("{}"), nil
	}
	ct, err := json.Marshal(r.Window.stored())
	return nil, ct, err
}

// Application is what one transaction applies to the index. Page holds the messages the provider
// reported, each added when the index lacks it and its labels and flags set when it holds it.
// Removed holds the messages the provider no longer holds. Cursor, when set, is stored with its write
// time in the same transaction.
type Application struct {
	Page    index.Page
	Removed []string
	Cursor  mail.Cursor
}

// Applied counts what an application changed. Inserted counts the messages it added, Changed those
// whose labels or flags it set, Removed those it removed, and Unclassified the added messages whose
// sender could not be classified.
type Applied struct {
	Inserted, Changed, Removed, Unclassified int
}

// Event is one event on a run's timeline, beside the start, progress, finish and failure events the
// Store records with the change they mark (ADR-0117).
type Event struct {
	// Kind is retry, and Detail the provider's error text or the tick's own, never a body.
	Kind, Detail string
}

// Item is one message whose body fetch the tick failed on, as job_run_failures records it.
type Item struct {
	ID string
	// Class is the error class, and Summary the provider's or the converter's error text, never a
	// body.
	Class, Summary string
	At             time.Time
	// Disposition is gone or abandoned.
	Disposition string
}

// Store is where a tick keeps the index and its runs. Each method is one transaction.
type Store interface {
	// State reads whether backfill's second pass has ended for the account, its change cursor with
	// its write time, and where the latest tick's scanning stopped.
	State(ctx context.Context, account string) (State, error)
	// Start records the run runID of the pass as it starts from at, with a start event. A tick's start
	// first records as failed, with a failure event, the account's latest tick and latest gap recovery
	// still recorded as running, in the same transaction.
	Start(ctx context.Context, account, runID, pass string, at Recorder) error
	// Apply makes one application durable. It removes the removed messages, adds each message the
	// index lacks with a masking event for each mask on its subject, sets the labels and flags of each
	// it holds, rebuilds the statistics of every sender the application touched and removes those of
	// a sender none of whose messages is stored, stores the cursor when one is set, and records the
	// progress record returns for what it applied, with a progress event, all in one transaction.
	Apply(ctx context.Context, account, runID string, a Application, record func(Applied) Recorder) (Applied, error)
	// Since returns the identifiers of the account's stored messages dated at or after start, in
	// their order.
	Since(ctx context.Context, account string, start time.Time) ([]string, error)
	// Delist runs the delisting transition. It reads the domains of the messages stored as restricted
	// or skipped as restricted, returns those messages of every domain delisted names to a normal
	// sender class and pending scan with its sender's statistics rebuilt, with an event when it marked
	// any, in one transaction (ADR-0037). It returns how many messages it marked.
	Delist(ctx context.Context, account, runID string, delisted func(restricted []string) []string) (int, error)
	// List restricts the stored classes a rule added since restricts. It reads the domains of the
	// messages stored as normal, sets those messages of every domain listed names to the restricted
	// class with the rule listed gives it, and rebuilds its sender's statistics, in one transaction
	// (ADR-0113). It leaves every scan state as it is. It returns how many messages it marked.
	List(ctx context.Context, account string, listed func(normal []string) []index.Listing) (int, error)
	// Pending returns up to n of the account's messages waiting for a scan after the message
	// identified by after, in the order of their identifiers.
	Pending(ctx context.Context, account, after string, n int) ([]index.Waiting, error)
	// CommitScan makes one read of waiting messages durable. It records the gate's decision on each
	// message the gate decided, the verdict of each scanned and the state of each skipped, adds the
	// flagged verdicts to their senders' prior hits, records the failed items, and records at with a
	// progress event, all in one transaction.
	CommitScan(ctx context.Context, account, runID string, outcomes []index.Outcome, items []Item, at Progress) error
	// Backlog returns how many of the account's messages wait for a scan.
	Backlog(ctx context.Context, account string) (int64, error)
	// Finish records at and that the run succeeded, with a finish event, and stores cursor with its
	// write time when it is set, in one transaction.
	Finish(ctx context.Context, account, runID string, cursor mail.Cursor, at Recorder) error
	// Fail records the run as failed with its last error, and a failure event.
	Fail(ctx context.Context, account, runID, cause string) error
	// Event adds an event to the run's timeline.
	Event(ctx context.Context, account, runID string, e Event) error
}
