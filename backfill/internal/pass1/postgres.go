package pass1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	maskingrecord "github.com/ppat/mediated-mailbox-mcp/db/maskingevents/record"
	messageingest "github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	"github.com/ppat/mediated-mailbox-mcp/db/senders"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// The run's workload and pass, as job_runs names them (ADR-0016).
const (
	workload = "backfill"
	pass     = "pass1"
)

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches.
func NewPostgres(db tx.Beginner) *Postgres { return &Postgres{db: db} }

// checkpoint and counters are the stored forms of a run's checkpoint and counters. The checkpoint's
// page is the page the jobs card shows, and its token is the provider's token for the next page. The
// port reports no total, so the checkpoint carries no of, and the card shows no estimate.
type checkpoint struct {
	Page  int    `json:"page"`
	Token string `json:"token"`
}

type counters struct {
	Pages    int `json:"pages"`
	Messages int `json:"messages"`
}

func encode(p core.Progress) (cp, ct []byte, err error) {
	cp, err = json.Marshal(checkpoint{Page: p.Checkpoint.Page, Token: string(p.Checkpoint.Token)})
	if err != nil {
		return nil, nil, err
	}
	ct, err = json.Marshal(counters{Pages: p.Counters.Pages, Messages: p.Counters.Messages})
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
		Checkpoint: core.Checkpoint{Page: c.Page, Token: mail.PageToken(c.Token)},
		Counters:   core.Counters{Pages: n.Pages, Messages: n.Messages},
	}, nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func page(n int) pgtype.Int4 { return pgtype.Int4{Int32: int32(min(n, 1<<31-1)), Valid: true} } //nolint:gosec // Bounded above.

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string) (bool, core.Latest, error) {
	var (
		ended  bool
		latest core.Latest
	)
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var err error
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the completion flag: %w", err)
		}
		ended = progress.BackfillPass1Complete
		row, err := record.New(t).LatestRun(ctx, record.LatestRunParams{AccountID: account, Workload: workload, Pass: text(pass)})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the latest run: %w", err)
		}
		latest = core.Latest{Found: true, RunID: row.RunID}
		switch row.State {
		case "running":
			latest.State = core.Running
		case "succeeded":
			latest.State = core.Succeeded
		case "failed":
			latest.State = core.Failed
		default:
			return fmt.Errorf("the run %s has the state %q", row.RunID, row.State)
		}
		latest.Progress, err = decode(row.Checkpoint, row.Counters)
		return err
	})
	return ended, latest, err
}

// Start implements Store.
func (s *Postgres) Start(ctx context.Context, account, runID string, start core.Start) error {
	cp, ct, err := encode(start.From)
	if err != nil {
		return err
	}
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		q := record.New(t)
		if start.Abandon {
			err := q.EndRun(ctx, record.EndRunParams{
				State: "failed", LastError: text("the run stopped before it recorded its end"), AccountID: account, RunID: start.ResumedFrom,
			})
			if err != nil {
				return fmt.Errorf("recording the stopped run: %w", err)
			}
		}
		err := q.StartRun(ctx, record.StartRunParams{
			AccountID: account, RunID: runID, Workload: workload, Pass: text(pass),
			ResumedFrom: text(start.ResumedFrom), Checkpoint: cp, Counters: ct,
		})
		if err != nil {
			return fmt.Errorf("recording the run: %w", err)
		}
		kind := "start"
		if start.ResumedFrom != "" {
			kind = "resume"
		}
		return q.RecordEvent(ctx, record.RecordEventParams{AccountID: account, RunID: runID, Kind: kind, Page: page(start.From.Checkpoint.Page)})
	})
}

// Commit implements Store.
func (s *Postgres) Commit(ctx context.Context, account, runID string, p core.Page, recovered *Failure, advance func(int) core.Progress) (Committed, error) {
	var c Committed
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		added, unclassified := 0, 0
		messages, events := messageingest.New(t), maskingrecord.New(t)
		for _, m := range p.Messages {
			ok, err := insert(ctx, messages, account, m)
			if err != nil {
				return fmt.Errorf("adding message %s: %w", m.ID, err)
			}
			if !ok {
				continue
			}
			added++
			if m.Unclassified {
				unclassified++
			}
			for _, mask := range m.Masks {
				err := events.RecordMaskingEvent(ctx, maskingrecord.RecordMaskingEventParams{
					AccountID: account, MessageID: m.ID, RuleID: mask.Rule, Tier: int32(mask.Tier), //nolint:gosec // A tier is 0, 1 or 2.
				})
				if err != nil {
					return fmt.Errorf("recording a masking event of message %s: %w", m.ID, err)
				}
			}
		}
		for _, domain := range p.Domains {
			err := senders.New(t).RebuildSender(ctx, senders.RebuildSenderParams{AccountID: account, Domain: domain})
			if err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", domain, err)
			}
		}
		q := record.New(t)
		if recovered != nil {
			if err := recordFailure(ctx, q, account, runID, *recovered); err != nil {
				return fmt.Errorf("recording the page's recovered failure: %w", err)
			}
		}
		at := advance(added)
		cp, ct, err := encode(at)
		if err != nil {
			return err
		}
		if err := q.RecordProgress(ctx, record.RecordProgressParams{Checkpoint: cp, Counters: ct, AccountID: account, RunID: runID}); err != nil {
			return fmt.Errorf("recording the checkpoint: %w", err)
		}
		if err := q.RecordEvent(ctx, record.RecordEventParams{AccountID: account, RunID: runID, Kind: "progress", Page: page(at.Checkpoint.Page)}); err != nil {
			return err
		}
		c = Committed{Progress: at, Unclassified: unclassified}
		return nil
	})
	return c, err
}

// insert adds one message to the index and reports whether it was added, false for a message the
// index already held.
func insert(ctx context.Context, q *messageingest.Queries, account string, m core.Message) (bool, error) {
	flags, err := json.Marshal(map[string]bool{"read": m.Flags.Read, "starred": m.Flags.Starred})
	if err != nil {
		return false, err
	}
	var auth []byte
	if m.AuthResults != (mail.AuthResults{}) {
		if auth, err = json.Marshal(map[string]string{"spf": m.AuthResults.SPF, "dkim": m.AuthResults.DKIM, "dmarc": m.AuthResults.DMARC}); err != nil {
			return false, err
		}
	}
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err = q.InsertMessage(ctx, messageingest.InsertMessageParams{
		AccountID:      account,
		MessageID:      m.ID,
		ThreadID:       m.ThreadID,
		FromEmail:      m.From.Email,
		FromDomain:     m.Domain,
		FromName:       text(m.From.Name),
		Subject:        pgtype.Text{String: m.Subject, Valid: true},
		SubjectMasked:  m.SubjectMasked,
		SentAt:         pgtype.Timestamptz{Time: time.UnixMilli(int64(m.Date)).UTC(), Valid: true},
		Labels:         labels,
		Flags:          flags,
		HasAttachments: m.HasAttachments,
		ListID:         text(m.ListID),
		SizeBytes:      pgtype.Int4{Int32: int32(min(max(m.SizeBytes, 0), 1<<31-1)), Valid: true}, //nolint:gosec // Bounded above.
		AuthResults:    auth,
		SenderClass:    string(m.Class),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := completion.New(t).SetBackfillFirstComplete(ctx, account); err != nil {
			return fmt.Errorf("setting the completion flag: %w", err)
		}
		q := record.New(t)
		if err := q.EndRun(ctx, record.EndRunParams{State: "succeeded", AccountID: account, RunID: runID}); err != nil {
			return fmt.Errorf("recording the run's end: %w", err)
		}
		return q.RecordEvent(ctx, record.RecordEventParams{AccountID: account, RunID: runID, Kind: "finish"})
	})
}

// Fail implements Store.
func (s *Postgres) Fail(ctx context.Context, account, runID, cause string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		q := record.New(t)
		if err := q.EndRun(ctx, record.EndRunParams{State: "failed", LastError: text(cause), AccountID: account, RunID: runID}); err != nil {
			return err
		}
		return q.RecordEvent(ctx, record.RecordEventParams{AccountID: account, RunID: runID, Kind: "failure", Detail: text(cause)})
	})
}

// Event implements Store.
func (s *Postgres) Event(ctx context.Context, account, runID string, e Event) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return record.New(t).RecordEvent(ctx, record.RecordEventParams{
			AccountID: account, RunID: runID, Kind: e.Kind, Page: page(e.Page), Detail: text(e.Detail),
		})
	})
}

// Failure implements Store.
func (s *Postgres) Failure(ctx context.Context, account, runID string, f Failure) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return recordFailure(ctx, record.New(t), account, runID, f)
	})
}

// recordFailure records one failed page in the transaction q runs in.
func recordFailure(ctx context.Context, q *record.Queries, account, runID string, f Failure) error {
	return q.RecordFailure(ctx, record.RecordFailureParams{
		AccountID:    account,
		RunID:        runID,
		ItemKind:     "page",
		ItemID:       fmt.Sprint(f.Page),
		Page:         page(f.Page),
		ErrorClass:   f.Class,
		ErrorSummary: text(f.Summary),
		Attempts:     int32(min(f.Attempts, 1<<31-1)), //nolint:gosec // Bounded above.
		FirstAt:      pgtype.Timestamptz{Time: f.First, Valid: true},
		LastAt:       pgtype.Timestamptz{Time: f.Last, Valid: true},
		Disposition:  f.Disposition,
	})
}
