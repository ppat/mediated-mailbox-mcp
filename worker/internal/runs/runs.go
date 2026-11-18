// Package runs records the runs of the worker's job kinds, once for both (ADR-0016, ADR-0117). Each
// function writes in the transaction the Queries it is handed run in, so a run's record becomes
// durable with the work it counts, in the transaction its job kind opened on its own pool (ADR-0143).
// A job kind names itself, its workload as job_runs spells it, at each call that reads or starts a
// run by it. The decisions about a run, how it starts from its latest recorded run and when a job's
// latest success is counted from, are worker/internal/runs/core's.
//
// A tick reads its latest runs, ends a run a stopped process left running, and writes the checkpoint
// of its progress and of its finish through db/jobruns/record's statements itself, rather than
// through LatestRun, FailRun and RecordProgress, so each error it returns or records reads as a
// tick's always has where this package words it for backfill. The events those writes mark still go
// through RecordEvent and SucceedRun.
//
// RecordedSuccess reads in a transaction of its own, since a job kind asks it at a job's ensure,
// outside any run.
package runs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core"
)

// Stopped is the last error a run a stopped process left running is recorded with.
const Stopped = "the run stopped before it recorded its end"

// NewID returns a new run's identifier, sixteen random hexadecimal digits.
func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}

// Page is the page an event or a failed item concerns, as the run's timeline and its failed items
// record it. The zero Page records none, a null page, as a delta sync tick records every event and
// item, and as every run records its finish and failure events. Backfill's runs record a page through
// OnPage on every other event and on every failed item, zero for one that concerns no page.
type Page struct {
	n   int
	set bool
}

// OnPage returns the page n, counted from one, or zero for a backfill event or failed item that
// concerns no page.
func OnPage(n int) Page { return Page{n: n, set: true} }

func (p Page) stored() pgtype.Int4 {
	return pgtype.Int4{Int32: int32(min(p.n, 1<<31-1)), Valid: p.set} //nolint:gosec // Bounded above.
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// Recorded is a pass's latest run as job_runs holds it, its checkpoint and counters still in their
// stored form, since each pass stores its own.
type Recorded struct {
	Found                bool
	RunID                string
	State                core.RunState
	Checkpoint, Counters []byte
}

// LatestRun reads the account's latest run of the workload's pass, in the transaction q runs in.
func LatestRun(ctx context.Context, q *runrecord.Queries, workload, pass string) (Recorded, error) {
	row, err := q.LatestRun(ctx, runrecord.LatestRunParams{Workload: workload, Pass: pass})
	if errors.Is(err, pgx.ErrNoRows) {
		return Recorded{}, nil
	}
	if err != nil {
		return Recorded{}, fmt.Errorf("reading the latest run: %w", err)
	}
	r := Recorded{Found: true, RunID: row.RunID, Checkpoint: row.Checkpoint, Counters: row.Counters}
	switch row.State {
	case "running":
		r.State = core.Running
	case "succeeded":
		r.State = core.Succeeded
	case "failed":
		r.State = core.Failed
	default:
		return Recorded{}, fmt.Errorf("the run %s has the state %q", row.RunID, row.State)
	}
	return r, nil
}

// Starting is a run as it starts, its checkpoint and counters in their stored form.
type Starting struct {
	Workload, Pass, RunID, ResumedFrom string
	// Abandon records the run resumed as failed first, with Stopped and no failure event, since it
	// stopped without recording its end.
	Abandon              bool
	Checkpoint, Counters []byte
	// Page is the checkpoint page the start or resume event carries.
	Page Page
}

// StartRun records a run as it starts, in the transaction q runs in, with a start event, or a resume
// event when it resumes a run.
func StartRun(ctx context.Context, q *runrecord.Queries, s Starting) error {
	if s.Abandon {
		err := q.EndRun(ctx, runrecord.EndRunParams{
			State: "failed", LastError: text(Stopped), RunID: s.ResumedFrom,
		})
		if err != nil {
			return fmt.Errorf("recording the stopped run: %w", err)
		}
	}
	err := q.StartRun(ctx, runrecord.StartRunParams{
		RunID: s.RunID, Workload: s.Workload, Pass: s.Pass,
		ResumedFrom: text(s.ResumedFrom), Checkpoint: s.Checkpoint, Counters: s.Counters,
	})
	if err != nil {
		return fmt.Errorf("recording the run: %w", err)
	}
	kind := "start"
	if s.ResumedFrom != "" {
		kind = "resume"
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{RunID: s.RunID, Kind: kind, Page: s.Page.stored()})
}

// RecordProgress records a running run's checkpoint and counters with a progress event on page, in
// the transaction that made the work they count durable.
func RecordProgress(ctx context.Context, q *runrecord.Queries, runID string, checkpoint, counters []byte, page Page) error {
	if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: checkpoint, Counters: counters, RunID: runID}); err != nil {
		return fmt.Errorf("recording the checkpoint: %w", err)
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{RunID: runID, Kind: "progress", Page: page.stored()})
}

// SucceedRun records that the run ended its pass, with a finish event.
func SucceedRun(ctx context.Context, q *runrecord.Queries, runID string) error {
	if err := q.EndRun(ctx, runrecord.EndRunParams{State: "succeeded", RunID: runID}); err != nil {
		return fmt.Errorf("recording the run's end: %w", err)
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{RunID: runID, Kind: "finish"})
}

// FailRun records the run as failed with its last error, and a failure event.
func FailRun(ctx context.Context, q *runrecord.Queries, runID, cause string) error {
	if err := q.EndRun(ctx, runrecord.EndRunParams{State: "failed", LastError: text(cause), RunID: runID}); err != nil {
		return err
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{RunID: runID, Kind: "failure", Detail: text(cause)})
}

// Event is one event on a run's timeline that a job kind records on its own. StartRun,
// RecordProgress, SucceedRun and FailRun record the start or resume, progress, finish and failure
// events with the change they mark, and a tick records the progress and failure events of the writes
// it makes itself as an Event (ADR-0117).
type Event struct {
	// Kind is backoff or retry, and for a tick's own writes also progress or failure.
	Kind string
	// Page is the page the event concerns.
	Page Page
	// Detail is the provider's error text or the job kind's own, never a body.
	Detail string
}

// RecordEvent adds an event to the run's timeline.
func RecordEvent(ctx context.Context, q *runrecord.Queries, runID string, e Event) error {
	return q.RecordEvent(ctx, runrecord.RecordEventParams{
		RunID: runID, Kind: e.Kind, Page: e.Page.stored(), Detail: text(e.Detail),
	})
}

// Item is one item a run failed on, as job_run_failures records it once the run knows what became of
// it (ADR-0016).
type Item struct {
	// Kind is page or message, and ID the page's number or the message's identifier.
	Kind, ID string
	// Page is the page the item was processed on.
	Page Page
	// Class is the error class, and Summary the provider's, the scanner's or the converter's last
	// error text, never a body.
	Class, Summary string
	// Attempts counts every attempt at the item, the one that recovered it included.
	Attempts    int
	First, Last time.Time
	// Disposition is recovered, gone, refused or abandoned.
	Disposition string
}

// RecordItem records one failed item in the transaction q runs in.
func RecordItem(ctx context.Context, q *runrecord.Queries, runID string, it Item) error {
	return q.RecordFailure(ctx, runrecord.RecordFailureParams{
		RunID:        runID,
		ItemKind:     it.Kind,
		ItemID:       it.ID,
		Page:         it.Page.stored(),
		ErrorClass:   it.Class,
		ErrorSummary: text(it.Summary),
		Attempts:     int32(min(it.Attempts, 1<<31-1)), //nolint:gosec // Bounded above.
		FirstAt:      pgtype.Timestamptz{Time: it.First, Valid: true},
		LastAt:       pgtype.Timestamptz{Time: it.Last, Valid: true},
		Disposition:  it.Disposition,
	})
}

// RecordedSuccess reads, in a transaction of its own on db, the account's latest recorded success of
// the workload, a run that succeeded or a step it made durable, and the start of its earliest run,
// each zero when its runs record none (ADR-0119).
func RecordedSuccess(ctx context.Context, db tx.Beginner, account, workload string) (latestSuccess, earliestStart mail.UnixMilli, err error) {
	var recorded runrecord.RecordedSuccessRow
	err = tx.Run(ctx, db, account, func(t pgx.Tx) error {
		var rErr error
		recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, workload)
		return rErr
	})
	if err != nil {
		return 0, 0, err
	}
	return millis(recorded.LatestSuccess), millis(recorded.EarliestStart), nil
}

// millis returns t in milliseconds since the epoch, and zero when it is null.
func millis(t pgtype.Timestamptz) mail.UnixMilli {
	if !t.Valid {
		return 0
	}
	return mail.UnixMilli(t.Time.UnixMilli())
}

// Time returns the time ms names, and the zero time for zero, which a job's ensure counts from.
func Time(ms mail.UnixMilli) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(ms))
}
