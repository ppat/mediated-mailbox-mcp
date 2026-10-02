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
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	maskingrecord "github.com/ppat/mediated-mailbox-mcp/db/maskingevents/record"
	messageingest "github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	"github.com/ppat/mediated-mailbox-mcp/db/senders/statistics"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// pass is the pass as job_runs names it (ADR-0016).
const pass = "pass1"

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches.
func NewPostgres(db tx.Beginner) *Postgres { return &Postgres{db: db} }

// checkpoint and counters are the stored forms of a run's checkpoint and counters. The checkpoint's
// page is the page the jobs card shows, and its token is the provider's token for the next page. Of
// is the pages the enumeration takes, left out when the provider reported no total, so the card shows
// no estimate, and a checkpoint stored without it reads as having none (ADR-0095). Its version and
// revision are the scanner the enumeration masks under (ADR-0096).
type checkpoint struct {
	Page     int    `json:"page"`
	Token    string `json:"token"`
	Of       int    `json:"of,omitempty"`
	Version  int    `json:"version"`
	Revision string `json:"revision"`
}

type counters struct {
	Pages    int `json:"pages"`
	Messages int `json:"messages"`
	Remasked int `json:"remasked"`
}

func encode(p core.Progress) (cp, ct []byte, err error) {
	c := p.Checkpoint
	cp, err = json.Marshal(checkpoint{Page: c.Page, Token: string(c.Token), Of: c.Of, Version: c.Stamp.Version, Revision: c.Stamp.Revision})
	if err != nil {
		return nil, nil, err
	}
	ct, err = json.Marshal(counters{Pages: p.Counters.Pages, Messages: p.Counters.Messages, Remasked: p.Counters.Remasked})
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
		Checkpoint: core.Checkpoint{Page: c.Page, Token: mail.PageToken(c.Token), Of: c.Of, Stamp: index.Stamp{Version: c.Version, Revision: c.Revision}},
		Counters:   core.Counters{Pages: n.Pages, Messages: n.Messages, Remasked: n.Remasked},
	}, nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func page(n int) pgtype.Int4 { return pgtype.Int4{Int32: int32(min(n, 1<<31-1)), Valid: true} } //nolint:gosec // Bounded above.

// version returns a scanner version as the database stores it.
func version(v int) int32 { return int32(min(max(v, 0), 1<<31-1)) } //nolint:gosec // Bounded.

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string, stamp index.Stamp) (bool, bool, core.Latest[core.Progress], error) {
	var (
		ended, due bool
		latest     core.Latest[core.Progress]
	)
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var err error
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the completion flag: %w", err)
		}
		ended = progress.BackfillPass1Complete
		due, err = messageingest.New(t).StaleSubject(ctx, messageingest.StaleSubjectParams{
			AccountID: account, ScannerVersion: version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("reading whether a subject was masked under another scanner: %w", err)
		}
		r, err := LatestRun(ctx, record.New(t), account, pass)
		if err != nil || !r.Found {
			return err
		}
		latest = core.Latest[core.Progress]{Found: true, RunID: r.RunID, State: r.State}
		latest.Progress, err = decode(r.Checkpoint, r.Counters)
		return err
	})
	return ended, due, latest, err
}

// Start implements Store.
func (s *Postgres) Start(ctx context.Context, account, runID string, start core.Start[core.Progress]) error {
	cp, ct, err := encode(start.From)
	if err != nil {
		return err
	}
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if start.Reopen {
			if err := completion.New(t).ReopenBackfillFirst(ctx, account); err != nil {
				return fmt.Errorf("recording the pass as due again: %w", err)
			}
		}
		return StartRun(ctx, record.New(t), account, Starting{
			Pass: pass, RunID: runID, ResumedFrom: start.ResumedFrom, Abandon: start.Abandon,
			Checkpoint: cp, Counters: ct, Page: start.From.Checkpoint.Page,
		})
	})
}

// Commit implements Store.
func (s *Postgres) Commit(ctx context.Context, account, runID string, p index.Page, recovered *Failure, advance func(int, int) core.Progress) (Committed, error) {
	var c Committed
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		added, remasked, unclassified := 0, 0, 0
		messages, events := messageingest.New(t), maskingrecord.New(t)
		for _, m := range p.Messages {
			ok, err := insert(ctx, messages, account, m)
			if err != nil {
				return fmt.Errorf("adding message %s: %w", m.ID, err)
			}
			if ok {
				added++
				if m.Unclassified {
					unclassified++
				}
			} else {
				if ok, err = remask(ctx, messages, account, m); err != nil {
					return fmt.Errorf("masking the subject of message %s again: %w", m.ID, err)
				}
				if !ok {
					continue
				}
				remasked++
			}
			if err := recordMasks(ctx, events, account, m); err != nil {
				return err
			}
		}
		for _, domain := range p.Domains {
			err := statistics.New(t).RebuildSender(ctx, statistics.RebuildSenderParams{AccountID: account, Domain: domain})
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
		at := advance(added, remasked)
		cp, ct, err := encode(at)
		if err != nil {
			return err
		}
		if err := RecordProgress(ctx, q, account, runID, cp, ct, at.Checkpoint.Page); err != nil {
			return err
		}
		c = Committed{Progress: at, Unclassified: unclassified}
		return nil
	})
	return c, err
}

// insert adds one message to the index and reports whether it was added, false for a message the
// index already held.
func insert(ctx context.Context, q *messageingest.Queries, account string, m index.Message) (bool, error) {
	flags, err := json.Marshal(index.StoredFlags(m.Flags))
	if err != nil {
		return false, err
	}
	var auth []byte
	if stored := index.StoredAuthResults(m.AuthResults); stored != nil {
		if auth, err = json.Marshal(stored); err != nil {
			return false, err
		}
	}
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err = q.InsertMessage(ctx, messageingest.InsertMessageParams{
		AccountID:              account,
		MessageID:              m.ID,
		ThreadID:               m.ThreadID,
		FromEmail:              m.From.Email,
		FromDomain:             m.Domain,
		FromName:               text(m.From.Name),
		Subject:                pgtype.Text{String: m.Subject, Valid: true},
		SubjectMasked:          m.SubjectMasked,
		SentAt:                 pgtype.Timestamptz{Time: time.UnixMilli(int64(m.Date)).UTC(), Valid: true},
		Labels:                 labels,
		Flags:                  flags,
		HasAttachments:         m.HasAttachments,
		ListID:                 text(m.ListID),
		SizeBytes:              pgtype.Int4{Int32: int32(min(max(m.SizeBytes, 0), 1<<31-1)), Valid: true}, //nolint:gosec // Bounded above.
		AuthResults:            auth,
		SenderClass:            string(m.Class),
		ClassRuleID:            text(m.ClassRule),
		SubjectScannerVersion:  pgtype.Int4{Int32: version(m.Stamp.Version), Valid: true},
		SubjectScannerRevision: pgtype.Text{String: m.Stamp.Revision, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// remask masks the subject of a message the index holds again, when its stored subject was masked
// under another scanner than m's, and reports whether it did.
func remask(ctx context.Context, q *messageingest.Queries, account string, m index.Message) (bool, error) {
	n, err := q.RemaskSubject(ctx, messageingest.RemaskSubjectParams{
		Subject: pgtype.Text{String: m.Subject, Valid: true}, SubjectMasked: m.SubjectMasked,
		ScannerVersion: version(m.Stamp.Version), ScannerRevision: m.Stamp.Revision,
		AccountID: account, MessageID: m.ID,
	})
	return n == 1, err
}

// recordMasks records a masking event for each mask on m's subject, with the scanner it was masked
// under (ADR-0003, ADR-0096).
func recordMasks(ctx context.Context, q *maskingrecord.Queries, account string, m index.Message) error {
	for _, mask := range m.Masks {
		err := q.RecordMaskingEvent(ctx, maskingrecord.RecordMaskingEventParams{
			AccountID: account, MessageID: m.ID, RuleID: mask.Rule, Tier: int32(mask.Tier), //nolint:gosec // A tier is 0, 1 or 2.
			ScannerVersion:  pgtype.Int4{Int32: version(m.Stamp.Version), Valid: true},
			ScannerRevision: pgtype.Text{String: m.Stamp.Revision, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("recording a masking event of message %s: %w", m.ID, err)
		}
	}
	return nil
}

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string, stamp index.Stamp, gone func([]core.Stored) []index.Message, now time.Time) (int, error) {
	whole := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		whole = 0
		messages, events, q := messageingest.New(t), maskingrecord.New(t), record.New(t)
		rows, err := messages.StaleSubjects(ctx, messageingest.StaleSubjectsParams{
			AccountID: account, ScannerVersion: version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("reading the subjects masked under another scanner: %w", err)
		}
		stored := make([]core.Stored, 0, len(rows))
		for _, r := range rows {
			stored = append(stored, core.Stored{ID: r.MessageID, Subject: r.Subject})
		}
		for _, m := range gone(stored) {
			ok, err := remask(ctx, messages, account, m)
			if err != nil {
				return fmt.Errorf("masking the subject of message %s whole: %w", m.ID, err)
			}
			if !ok {
				return fmt.Errorf("masking the subject of message %s whole changed no row", m.ID)
			}
			if err := recordMasks(ctx, events, account, m); err != nil {
				return err
			}
			err = RecordItem(ctx, q, account, runID, Item{
				Kind: "message", ID: m.ID, Class: string(index.Gone), Attempts: 1, First: now, Last: now, Disposition: "gone",
				Summary: "the enumeration did not find the message, so its subject was masked whole under the scanner in force",
			})
			if err != nil {
				return fmt.Errorf("recording the message %s as gone: %w", m.ID, err)
			}
			whole++
		}
		if err := completion.New(t).SetBackfillFirstComplete(ctx, account); err != nil {
			return fmt.Errorf("setting the completion flag: %w", err)
		}
		return SucceedRun(ctx, q, account, runID)
	})
	return whole, err
}

// Fail implements Store.
func (s *Postgres) Fail(ctx context.Context, account, runID, cause string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return FailRun(ctx, record.New(t), account, runID, cause)
	})
}

// Event implements Store.
func (s *Postgres) Event(ctx context.Context, account, runID string, e Event) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return RecordEvent(ctx, record.New(t), account, runID, e)
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
	return RecordItem(ctx, q, account, runID, Item{
		Kind: "page", ID: fmt.Sprint(f.Page), Page: f.Page, Class: f.Class, Summary: f.Summary,
		Attempts: f.Attempts, First: f.First, Last: f.Last, Disposition: f.Disposition,
	})
}
