package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/cursor"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	maskingrecord "github.com/ppat/mediated-mailbox-mcp/db/maskingevents/record"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/change"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/scan"
	gaterecord "github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions/record"
	"github.com/ppat/mediated-mailbox-mcp/db/senders/statistics"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	workerindex "github.com/ppat/mediated-mailbox-mcp/worker/internal/index"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/runs"
)

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
	// kind is the job kind the ticks and recoveries are recorded under, as job_runs.workload spells it.
	kind string
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches, recording its runs under the job kind
// kind.
func NewPostgres(db tx.Beginner, kind string) *Postgres { return &Postgres{db: db, kind: kind} }

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string) (State, error) {
	var st State
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		st = State{}
		progress, err := accountstate.New(t).AccountProgress(ctx)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading the account's progress: %w", err)
		}
		st.SecondEnded = progress.BackfillPass2Complete
		c, err := cursor.New(t).SyncCursor(ctx)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading the change cursor: %w", err)
		}
		st.Cursor = mail.Cursor(c.SyncCursor.String)
		if c.SyncCursorAt.Valid {
			at := c.SyncCursorAt.Time.UTC()
			st.CursorAt = &at
		}
		latest, err := runrecord.New(t).LatestRun(ctx, runrecord.LatestRunParams{Workload: s.kind, Pass: PassTick})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the latest tick: %w", err)
		}
		var cp struct {
			After string `json:"after"`
		}
		if len(latest.Checkpoint) > 0 {
			if err := json.Unmarshal(latest.Checkpoint, &cp); err != nil {
				return fmt.Errorf("reading the latest tick's checkpoint: %w", err)
			}
		}
		st.ScanAfter = cp.After
		return nil
	})
	return st, err
}

// Start implements Store.
func (s *Postgres) Start(ctx context.Context, account, runID, pass string, at Recorder) error {
	cp, ct, err := at.encode()
	if err != nil {
		return err
	}
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		q := runrecord.New(t)
		if pass == PassTick {
			if err := s.endStopped(ctx, q); err != nil {
				return err
			}
		}
		return runs.StartRun(ctx, q, runs.Starting{Workload: s.kind, Pass: pass, RunID: runID, Checkpoint: cp, Counters: ct})
	})
}

// endStopped records as failed, with a failure event, the account's latest tick and latest gap
// recovery while either is still recorded as running. Ticks do not overlap, so one still running when
// a tick starts was stopped before it recorded its end (ADR-0103).
//
// It runs the statements itself to keep the error text a run records, which runs.LatestRun and
// runs.FailRun word for backfill.
func (s *Postgres) endStopped(ctx context.Context, q *runrecord.Queries) error {
	for _, pass := range []string{PassTick, PassGapRecovery} {
		latest, err := q.LatestRun(ctx, runrecord.LatestRunParams{Workload: s.kind, Pass: pass})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading the latest %s run: %w", pass, err)
		}
		if latest.State != "running" {
			continue
		}
		stopped := pgtype.Text{String: runs.Stopped, Valid: true}
		if err := q.EndRun(ctx, runrecord.EndRunParams{State: "failed", LastError: stopped, RunID: latest.RunID}); err != nil {
			return fmt.Errorf("recording the stopped %s run: %w", pass, err)
		}
		if err := runs.RecordEvent(ctx, q, latest.RunID, runs.Event{Kind: "failure", Detail: runs.Stopped}); err != nil {
			return err
		}
	}
	return nil
}

// Apply implements Store.
func (s *Postgres) Apply(ctx context.Context, account, runID string, a Application, progress func(Applied) Recorder) (Applied, error) {
	var applied Applied
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		applied = Applied{}
		touched := map[string]bool{}
		if len(a.Removed) > 0 {
			domains, err := change.New(t).RemoveMessages(ctx, a.Removed)
			if err != nil {
				return fmt.Errorf("removing the messages the provider no longer holds: %w", err)
			}
			applied.Removed = len(domains)
			for _, d := range domains {
				touched[d] = true
			}
		}
		messages, events, changes := ingest.New(t), maskingrecord.New(t), change.New(t)
		for _, m := range a.Page.Messages {
			added, err := workerindex.Insert(ctx, messages, m)
			if err != nil {
				return fmt.Errorf("adding message %s: %w", m.ID, err)
			}
			touched[m.Domain] = true
			if added {
				applied.Inserted++
				if m.Unclassified {
					applied.Unclassified++
				}
				if err := workerindex.RecordMasks(ctx, events, m); err != nil {
					return err
				}
				continue
			}
			changed, err := relabel(ctx, changes, m)
			if err != nil {
				return fmt.Errorf("setting the labels and flags of message %s: %w", m.ID, err)
			}
			if changed {
				applied.Changed++
			}
		}
		for _, d := range slices.Sorted(maps.Keys(touched)) {
			if err := statistics.New(t).RebuildSender(ctx, d); err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", d, err)
			}
			if _, err := changes.DropEmptySender(ctx, d); err != nil {
				return fmt.Errorf("removing the statistics of the sender at %q: %w", d, err)
			}
		}
		if a.Cursor != "" {
			if err := advance(ctx, cursor.New(t), a.Cursor); err != nil {
				return err
			}
		}
		return recordProgress(ctx, runrecord.New(t), runID, progress(applied))
	})
	return applied, err
}

// advance stores the account's change cursor with its write time.
func advance(ctx context.Context, q *cursor.Queries, c mail.Cursor) error {
	n, err := q.AdvanceCursor(ctx, pgtype.Text{String: string(c), Valid: true})
	if err != nil {
		return fmt.Errorf("storing the change cursor: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("storing the change cursor changed %d rows, want 1", n)
	}
	return nil
}

// relabel sets a stored message's labels and flags to the ones the provider reports, and reports
// whether they changed.
func relabel(ctx context.Context, q *change.Queries, m index.Message) (bool, error) {
	flags, err := json.Marshal(index.StoredFlags(m.Flags))
	if err != nil {
		return false, err
	}
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	n, err := q.ApplyLabelsAndFlags(ctx, change.ApplyLabelsAndFlagsParams{Labels: labels, Flags: flags, MessageID: m.ID})
	return n == 1, err
}

// recordProgress records a running run's checkpoint and counters with a progress event naming no
// page, in the transaction that made the work they count durable.
//
// It runs the checkpoint's statement itself to keep the error text a run records, which
// runs.RecordProgress words for backfill.
func recordProgress(ctx context.Context, q *runrecord.Queries, runID string, at Recorder) error {
	cp, ct, err := at.encode()
	if err != nil {
		return err
	}
	if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: cp, Counters: ct, RunID: runID}); err != nil {
		return fmt.Errorf("recording the run's progress: %w", err)
	}
	return runs.RecordEvent(ctx, q, runID, runs.Event{Kind: "progress"})
}

// Since implements Store.
func (s *Postgres) Since(ctx context.Context, account string, start time.Time) ([]string, error) {
	var out []string
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var err error
		out, err = change.New(t).MessagesSince(ctx, pgtype.Timestamptz{Time: start, Valid: true})
		return err
	})
	return out, err
}

// Delist implements Store.
func (s *Postgres) Delist(ctx context.Context, account, runID string, delisted func([]string) []string) (int, error) {
	marked := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var senders int
		var err error
		marked, senders, err = workerindex.Delist(ctx, scan.New(t), delisted)
		if err != nil {
			return err
		}
		if marked == 0 {
			return nil
		}
		return runs.RecordEvent(ctx, runrecord.New(t), runID, runs.Event{
			Kind: "retry",
			Detail: fmt.Sprintf("the delisting transition returned %d messages of %d senders the policy no longer restricts to pending scan, "+
				"so the scanning starts from the first message waiting for a scan", marked, senders),
		})
	})
	return marked, err
}

// Pending implements Store.
func (s *Postgres) Pending(ctx context.Context, account, after string, n int) ([]index.Waiting, error) {
	var out []index.Waiting
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		out = nil
		rows, err := scan.New(t).PendingPage(ctx, scan.PendingPageParams{After: after, PageSize: int32(min(max(n, 1), 1<<31-1))}) //nolint:gosec // Bounded.
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, index.Waiting{
				ID: r.MessageID, From: r.FromEmail, Domain: r.FromDomain, SubjectMasked: r.SubjectMasked, ListID: r.HasListID,
				SizeBytes: r.SizeBytes, SentAt: mail.UnixMilli(r.SentAt.Time.UnixMilli()), SenderVolume: r.SenderVolume, SenderHits: r.SenderHits,
			})
		}
		return nil
	})
	return out, err
}

// CommitScan implements Store.
func (s *Postgres) CommitScan(ctx context.Context, account, runID string, outcomes []index.Outcome, items []runs.Item, at Progress) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := workerindex.RecordOutcomes(ctx, scan.New(t), gaterecord.New(t), statistics.New(t), outcomes); err != nil {
			return err
		}
		q := runrecord.New(t)
		for _, it := range items {
			if err := runs.RecordItem(ctx, q, runID, it); err != nil {
				return fmt.Errorf("recording the failed message %s: %w", it.ID, err)
			}
		}
		return recordProgress(ctx, q, runID, at)
	})
}

// Backlog implements Store.
func (s *Postgres) Backlog(ctx context.Context, account string) (int64, error) {
	return workerindex.Backlog(ctx, s.db, account)
}

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string, c mail.Cursor, at Recorder) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if c != "" {
			if err := advance(ctx, cursor.New(t), c); err != nil {
				return err
			}
		}
		q := runrecord.New(t)
		cp, ct, err := at.encode()
		if err != nil {
			return err
		}
		if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: cp, Counters: ct, RunID: runID}); err != nil {
			return fmt.Errorf("recording the run's progress: %w", err)
		}
		return runs.SucceedRun(ctx, q, runID)
	})
}

// Fail implements Store.
func (s *Postgres) Fail(ctx context.Context, account, runID, cause string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return runs.FailRun(ctx, runrecord.New(t), runID, cause)
	})
}

// Event implements Store.
func (s *Postgres) Event(ctx context.Context, account, runID string, e runs.Event) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return runs.RecordEvent(ctx, runrecord.New(t), runID, e)
	})
}
