package pass2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/scan"
	gaterecord "github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions/record"
	"github.com/ppat/mediated-mailbox-mcp/db/senders/statistics"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core"
	workerindex "github.com/ppat/mediated-mailbox-mcp/worker/internal/index"
	indexcore "github.com/ppat/mediated-mailbox-mcp/worker/internal/index/core"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/runs"
	runscore "github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core"
)

// pass is the pass as job_runs names it (ADR-0016).
const pass = "pass2"

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
	// kind is the job kind the pass's runs are recorded under, as job_runs.workload spells it.
	kind string
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches, recording its runs under the job kind
// kind.
func NewPostgres(db tx.Beginner, kind string) *Postgres { return &Postgres{db: db, kind: kind} }

// checkpoint and counters are the stored forms of a run's checkpoint and counters. The checkpoint's
// page is the page the jobs card shows, and after and sent_at are the position of the last message the
// durable pages read, its identifier and its send time in milliseconds since the Unix epoch. The
// position a run holds between pages carries milliseconds too, and both name a stored send time
// exactly because every writer of messages that ships stores whole milliseconds. A writer of a finer
// send time would let a page skip waiting messages, which stay pending and denied (ADR-0126).
type checkpoint struct {
	Page   int    `json:"page"`
	After  string `json:"after"`
	SentAt int64  `json:"sent_at"`
}

type counters struct {
	Pages   int `json:"pages"`
	Decided int `json:"decided"`
	Pending int `json:"pending"`
	Scanned int `json:"scanned"`
	Skipped int `json:"skipped"`
	Refused int `json:"refused"`
}

func encode(p core.Progress) (cp, ct []byte, err error) {
	after := p.Checkpoint.After
	cp, err = json.Marshal(checkpoint{Page: p.Checkpoint.Page, After: after.ID, SentAt: int64(after.SentAt)})
	if err != nil {
		return nil, nil, err
	}
	c := p.Counters
	ct, err = json.Marshal(counters{Pages: c.Pages, Decided: c.Decided, Pending: c.Pending, Scanned: c.Scanned, Skipped: c.Skipped, Refused: c.Refused})
	return cp, ct, err
}

func decode(cp, ct []byte) (core.Progress, error) {
	var c checkpoint
	var n counters
	if len(cp) > 0 {
		if err := json.Unmarshal(cp, &c); err != nil {
			return core.Progress{}, fmt.Errorf("reading the checkpoint: %w", err)
		}
	}
	if err := json.Unmarshal(ct, &n); err != nil {
		return core.Progress{}, fmt.Errorf("reading the counters: %w", err)
	}
	return core.Progress{
		Checkpoint: core.Checkpoint{Page: c.Page, After: core.Position{SentAt: mail.UnixMilli(c.SentAt), ID: c.After}},
		Counters: core.Counters{Pages: n.Pages, Counts: indexcore.Counts{
			Decided: n.Decided, Pending: n.Pending, Scanned: n.Scanned, Skipped: n.Skipped, Refused: n.Refused,
		}},
	}, nil
}

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string) (bool, bool, bool, runscore.Latest[core.Progress], error) {
	var (
		first, ended, restart bool
		latest                runscore.Latest[core.Progress]
	)
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		progress, err := accountstate.New(t).AccountProgress(ctx)
		if err != nil {
			return fmt.Errorf("reading the completion flags: %w", err)
		}
		first, ended = progress.BackfillPass1Complete, progress.BackfillPass2Complete
		if restart, err = completion.New(t).SecondRestart(ctx); err != nil {
			return fmt.Errorf("reading whether the pass is marked to start over: %w", err)
		}
		r, err := runs.LatestRun(ctx, runrecord.New(t), s.kind, pass)
		if err != nil || !r.Found {
			return err
		}
		latest = runscore.Latest[core.Progress]{Found: true, RunID: r.RunID, State: r.State}
		latest.Progress, err = decode(r.Checkpoint, r.Counters)
		return err
	})
	return first, ended, restart, latest, err
}

// Start implements Store.
func (s *Postgres) Start(ctx context.Context, account, runID string, start runscore.Start[core.Progress]) error {
	cp, ct, err := encode(start.From)
	if err != nil {
		return err
	}
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := completion.New(t).ClearSecondRestart(ctx); err != nil {
			return fmt.Errorf("clearing the mark that starts the pass over: %w", err)
		}
		return runs.StartRun(ctx, runrecord.New(t), runs.Starting{
			Workload: s.kind, Pass: pass, RunID: runID, ResumedFrom: start.ResumedFrom, Abandon: start.Abandon,
			Checkpoint: cp, Counters: ct, Page: runs.OnPage(start.From.Checkpoint.Page),
		})
	})
}

// Delist implements Store.
func (s *Postgres) Delist(ctx context.Context, account, runID string, delisted func([]string) []string, restart core.Progress) (int, error) {
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
		cp, ct, err := encode(restart)
		if err != nil {
			return err
		}
		r := runrecord.New(t)
		if err := runs.RecordProgress(ctx, r, runID, cp, ct, runs.OnPage(restart.Checkpoint.Page)); err != nil {
			return err
		}
		return runs.RecordEvent(ctx, r, runID, runs.Event{
			Kind: "retry", Page: runs.OnPage(0),
			Detail: fmt.Sprintf("the delisting transition returned %d messages of %d senders the policy no longer restricts to pending scan, "+
				"so the pass starts over from the first message waiting for a scan", marked, senders),
		})
	})
	return marked, err
}

// RequeueSkips implements Store.
func (s *Postgres) RequeueSkips(ctx context.Context, account, runID string) (int, error) {
	marked := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		skips, err := scan.New(t).RequeueSignalledSkips(ctx)
		if err != nil {
			return fmt.Errorf("returning the skips decided without their subject's signal to pending scan: %w", err)
		}
		marked = int(skips)
		if marked == 0 {
			return nil
		}
		return runs.RecordEvent(ctx, runrecord.New(t), runID, runs.Event{
			Kind: "retry", Page: runs.OnPage(0),
			Detail: fmt.Sprintf("%d skips decided without their subject's signal returned to pending scan", skips),
		})
	})
	return marked, err
}

// Pending implements Store. The zero position reads from the position of an infinite send time and
// an empty identifier, which every message sorts before, so the first page runs the statement every
// later page runs (ADR-0126).
func (s *Postgres) Pending(ctx context.Context, account string, after core.Position, n int) ([]index.Waiting, error) {
	from := pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}
	if after.ID != "" {
		from = pgtype.Timestamptz{Time: time.UnixMilli(int64(after.SentAt)).UTC(), Valid: true}
	}
	var out []index.Waiting
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		out = nil
		rows, err := scan.New(t).PendingPageNewestFirst(ctx, scan.PendingPageNewestFirstParams{
			AfterSentAt: from, AfterMessageID: after.ID, PageSize: int32(min(max(n, 1), 1<<31-1)), //nolint:gosec // Bounded.
		})
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

// Commit implements Store.
func (s *Postgres) Commit(ctx context.Context, account, runID string, p core.Page, items []runs.Item, at core.Progress) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := workerindex.RecordOutcomes(ctx, scan.New(t), gaterecord.New(t), statistics.New(t), p.Outcomes); err != nil {
			return err
		}
		r := runrecord.New(t)
		for _, it := range items {
			if err := runs.RecordItem(ctx, r, runID, it); err != nil {
				return fmt.Errorf("recording the failed message %s: %w", it.ID, err)
			}
		}
		cp, ct, err := encode(at)
		if err != nil {
			return err
		}
		return runs.RecordProgress(ctx, r, runID, cp, ct, runs.OnPage(at.Checkpoint.Page))
	})
}

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := completion.New(t).SetBackfillSecondComplete(ctx); err != nil {
			return fmt.Errorf("setting the completion flag: %w", err)
		}
		return runs.SucceedRun(ctx, runrecord.New(t), runID)
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

// Failure implements Store.
func (s *Postgres) Failure(ctx context.Context, account, runID string, it runs.Item) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return runs.RecordItem(ctx, runrecord.New(t), runID, it)
	})
}

// Backlog implements Store.
func (s *Postgres) Backlog(ctx context.Context, account string) (int64, error) {
	return workerindex.Backlog(ctx, s.db, account)
}
