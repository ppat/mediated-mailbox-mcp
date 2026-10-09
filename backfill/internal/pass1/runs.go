package pass1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
)

// Workload is backfill as job_runs names it (ADR-0016).
const Workload = "backfill"

// Recorded is a pass's latest run as job_runs holds it, its checkpoint and counters still in their
// stored form, since each pass stores its own.
type Recorded struct {
	Found                bool
	RunID                string
	State                core.RunState
	Checkpoint, Counters []byte
}

// LatestRun reads the account's latest run of the pass, in the transaction q runs in.
func LatestRun(ctx context.Context, q *runrecord.Queries, account, pass string) (Recorded, error) {
	row, err := q.LatestRun(ctx, runrecord.LatestRunParams{AccountID: account, Workload: Workload, Pass: pass})
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
	Pass, RunID, ResumedFrom string
	// Abandon records the run resumed as failed first, since it stopped without recording its end.
	Abandon              bool
	Checkpoint, Counters []byte
	// Page is the checkpoint page the start or resume event carries.
	Page int
}

// StartRun records a run as it starts, in the transaction q runs in, with a start event, or a resume
// event naming the checkpoint page.
func StartRun(ctx context.Context, q *runrecord.Queries, account string, s Starting) error {
	if s.Abandon {
		err := q.EndRun(ctx, runrecord.EndRunParams{
			State: "failed", LastError: text("the run stopped before it recorded its end"), AccountID: account, RunID: s.ResumedFrom,
		})
		if err != nil {
			return fmt.Errorf("recording the stopped run: %w", err)
		}
	}
	err := q.StartRun(ctx, runrecord.StartRunParams{
		AccountID: account, RunID: s.RunID, Workload: Workload, Pass: s.Pass,
		ResumedFrom: text(s.ResumedFrom), Checkpoint: s.Checkpoint, Counters: s.Counters,
	})
	if err != nil {
		return fmt.Errorf("recording the run: %w", err)
	}
	kind := "start"
	if s.ResumedFrom != "" {
		kind = "resume"
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: s.RunID, Kind: kind, Page: page(s.Page)})
}

// RecordProgress records a running run's checkpoint and counters with a progress event naming the
// checkpoint page, in the transaction that made the work they count durable.
func RecordProgress(ctx context.Context, q *runrecord.Queries, account, runID string, checkpoint, counters []byte, at int) error {
	if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: checkpoint, Counters: counters, AccountID: account, RunID: runID}); err != nil {
		return fmt.Errorf("recording the checkpoint: %w", err)
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "progress", Page: page(at)})
}

// SucceedRun records that the run ended its pass, with a finish event.
func SucceedRun(ctx context.Context, q *runrecord.Queries, account, runID string) error {
	if err := q.EndRun(ctx, runrecord.EndRunParams{State: "succeeded", AccountID: account, RunID: runID}); err != nil {
		return fmt.Errorf("recording the run's end: %w", err)
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "finish"})
}

// FailRun records the run as failed with its last error, and a failure event.
func FailRun(ctx context.Context, q *runrecord.Queries, account, runID, cause string) error {
	if err := q.EndRun(ctx, runrecord.EndRunParams{State: "failed", LastError: text(cause), AccountID: account, RunID: runID}); err != nil {
		return err
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "failure", Detail: text(cause)})
}

// RecordEvent adds an event to the run's timeline.
func RecordEvent(ctx context.Context, q *runrecord.Queries, account, runID string, e Event) error {
	return q.RecordEvent(ctx, runrecord.RecordEventParams{
		AccountID: account, RunID: runID, Kind: e.Kind, Page: page(e.Page), Detail: text(e.Detail),
	})
}

// Item is one item a run failed on, as job_run_failures records it once the run knows what became of
// it (ADR-0016).
type Item struct {
	// Kind is page or message, and ID the page's number or the message's identifier.
	Kind, ID string
	// Page is the page the item was processed on, counted from one.
	Page int
	// Class is the error class, and Summary the provider's, the scanner's or the converter's last
	// error text, never a body.
	Class, Summary string
	// Attempts counts every attempt at the item, the one that recovered it included.
	Attempts    int
	First, Last time.Time
	// Disposition is recovered, gone or abandoned.
	Disposition string
}

// RecordItem records one failed item in the transaction q runs in.
func RecordItem(ctx context.Context, q *runrecord.Queries, account, runID string, it Item) error {
	return q.RecordFailure(ctx, runrecord.RecordFailureParams{
		AccountID:    account,
		RunID:        runID,
		ItemKind:     it.Kind,
		ItemID:       it.ID,
		Page:         page(it.Page),
		ErrorClass:   it.Class,
		ErrorSummary: text(it.Summary),
		Attempts:     int32(min(it.Attempts, 1<<31-1)), //nolint:gosec // Bounded above.
		FirstAt:      pgtype.Timestamptz{Time: it.First, Valid: true},
		LastAt:       pgtype.Timestamptz{Time: it.Last, Valid: true},
		Disposition:  it.Disposition,
	})
}
