// Package start is backfill's run-start step, which a job makes once for its life, before the first
// pass of the first run that reaches it (ADR-0120, ADR-0121). It returns to pending every verdict made
// under another scanner than the one the job scans with and every body refused under another size
// bound than the port's (ADR-0132), masks again from the store every subject stored unmasked under
// another scanner, and returns to pending every stored gate skip the gate no longer decides as the
// same skip. Its decision on a skip is worker/internal/backfill/start/core's, and a subject is masked
// again as the first pass masks it. It writes to no run's record.
package start

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	maskingrecord "github.com/ppat/mediated-mailbox-mcp/db/maskingevents/record"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	scanstore "github.com/ppat/mediated-mailbox-mcp/db/messages/scan"
	"github.com/ppat/mediated-mailbox-mcp/db/senders/statistics"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	pass1core "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/start/core"
	workerindex "github.com/ppat/mediated-mailbox-mcp/worker/internal/index"
)

// Store is where the step finds and changes what it returns to pending and masks again.
type Store interface {
	// Reopen returns to pending scan each scanned message whose verdict was made under another scanner
	// than the one stamped s, its verdict cleared, and counts again the prior hits of their senders.
	// It then reads, in batches, every subject stored unmasked and masked under another scanner, and
	// stores the subjects remasked returns for them under s with a masking event for each mask, a batch
	// in a few statements. It then reads every message the gate skipped with its gate inputs and
	// returns to pending scan each one overturned names. When it marked any message, masked any
	// subject again, or left a stored subject masked under another scanner so the first pass is due
	// again, it records the second pass as not ended and marks it to start over. It writes to no run's
	// record. It is one transaction (ADR-0120, ADR-0121). It returns how many of each it marked or
	// masked again.
	Reopen(ctx context.Context, account string, s index.Stamp, remasked func(stored []pass1core.Stored) []index.Message, overturned func(skips []index.Waiting) []string) (Reopened, error)
}

// Deps are what the step runs with.
type Deps struct {
	Store Store
	// Policy returns the account's policy in the active snapshot, with the classifier's domain
	// functions it was prepared with. Gate is the scan gate's thresholds and Scanner the scanner the
	// job scans with.
	Policy  func() classify.Policy
	Gate    scangate.Config
	Scanner scan.Scanner
	// Now is the clock a skipped message's age is measured at.
	Now func() time.Time
}

// Reopened counts what the step returned to pending, the verdicts made under another scanner, the
// bodies refused under another size bound (ADR-0132) and the gate skips the gate no longer decides as
// the same skip, and the subjects stored unmasked it masked again from the store.
type Reopened struct {
	Verdicts, Refusals, Skips, Subjects int
}

// Step makes the run-start step over the account. It returns to pending every verdict made under
// another scanner than the one the job scans with and every body refused under another size bound than
// the port's, masks again from the store, with that scanner, every subject stored unmasked under
// another, and then returns to pending every stored gate skip the gate, deciding it again under the
// job's thresholds and policy with the message's age measured at the job's clock, no longer decides as
// the same skip, so those decisions see the subjects as now masked. It reopens the second pass when it
// did any of these or when the first pass is due again, so a stale verdict or an overturned skip is
// denied from the start of the run that sees it, whatever the first pass's state. It logs what it
// did, as counts (ADR-0120, ADR-0121).
func Step(ctx context.Context, deps Deps, account string, logger *slog.Logger) (Reopened, error) {
	now := mail.UnixMilli(deps.Now().UnixMilli())
	composed := deps.Policy()
	r, err := deps.Store.Reopen(ctx, account, index.StampOf(deps.Scanner), func(stored []pass1core.Stored) []index.Message {
		return pass1core.Remasked(stored, deps.Scanner)
	}, func(skips []index.Waiting) []string {
		return core.Overturned(composed, deps.Gate, now, skips)
	})
	if err != nil {
		return Reopened{}, fmt.Errorf("returning the verdicts another scanner made and the overturned gate skips to pending, and masking the subjects stored unmasked again: %w", err)
	}
	if r.Refusals > 0 {
		logger.Info("bodies refused under another size bound returned to pending", "messages", r.Refusals)
	}
	if r.Verdicts > 0 {
		logger.Info("verdicts made under another scanner returned to pending", "messages", r.Verdicts)
	}
	if r.Subjects > 0 {
		logger.Info("subjects stored unmasked under another scanner masked again from the store", "messages", r.Subjects)
	}
	if r.Skips > 0 {
		logger.Info("gate skips the gate no longer decides as the same skip returned to pending", "messages", r.Skips)
	}
	return r, nil
}

// remaskBatch is how many subjects stored unmasked the step reads, masks again and writes back in one
// batch of statements, so a mailbox's subjects are masked again in a few statements rather than one a
// message, while what one batch holds in memory stays small (ADR-0120).
const remaskBatch = 5000

// Postgres is the Store in the index's database. Its one method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches.
func NewPostgres(db tx.Beginner) *Postgres { return &Postgres{db: db} }

// Reopen implements Store.
func (s *Postgres) Reopen(ctx context.Context, account string, stamp index.Stamp, remasked func([]pass1core.Stored) []index.Message, overturned func([]index.Waiting) []string) (Reopened, error) {
	var marked Reopened
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		marked = Reopened{}
		domains, err := scanstore.New(t).RequeueStaleVerdicts(ctx, scanstore.RequeueStaleVerdictsParams{
			ScannerVersion: workerindex.Version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("returning the verdicts another scanner made to pending scan: %w", err)
		}
		slices.Sort(domains)
		for _, d := range slices.Compact(slices.Clone(domains)) {
			n, err := statistics.New(t).RecountScanHits(ctx, d)
			if err != nil {
				return fmt.Errorf("counting the prior hits of the sender at %q again: %w", d, err)
			}
			if n != 1 {
				return fmt.Errorf("counting the prior hits of the sender at %q again changed %d rows, want 1", d, n)
			}
		}
		refusals, err := scanstore.New(t).RequeueRefusals(ctx, workerindex.RefusedBound)
		if err != nil {
			return fmt.Errorf("returning the bodies refused under another size bound to pending scan: %w", err)
		}
		subjects, err := remaskStored(ctx, ingest.New(t), maskingrecord.New(t), stamp, remasked)
		if err != nil {
			return err
		}
		skips, err := requeueOverturned(ctx, scanstore.New(t), overturned)
		if err != nil {
			return err
		}
		stale, err := ingest.New(t).StaleSubject(ctx, ingest.StaleSubjectParams{
			ScannerVersion: workerindex.Version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("reading whether a subject was masked under another scanner: %w", err)
		}
		marked = Reopened{Verdicts: len(domains), Refusals: int(refusals), Skips: skips, Subjects: subjects}
		if marked.Verdicts == 0 && marked.Refusals == 0 && marked.Skips == 0 && marked.Subjects == 0 && !stale {
			return nil
		}
		if err := completion.New(t).ReopenBackfillSecond(ctx); err != nil {
			return fmt.Errorf("recording the pass as due again: %w", err)
		}
		return nil
	})
	return marked, err
}

// remaskStored masks again every stale subject the account stores unmasked, from the subject stored,
// in the transaction messages and events run in, a batch at a time in the order of the messages'
// identifiers. Each batch is a read, the subjects remasked returns written back with the pair stamp in
// one statement, and their masking events recorded in one more (ADR-0120). It returns how many
// subjects it masked again.
func remaskStored(ctx context.Context, messages *ingest.Queries, events *maskingrecord.Queries, stamp index.Stamp,
	remasked func([]pass1core.Stored) []index.Message,
) (int, error) {
	v := workerindex.Version(stamp.Version)
	total, after := 0, ""
	for {
		rows, err := messages.StaleUnmaskedSubjects(ctx, ingest.StaleUnmaskedSubjectsParams{
			ScannerVersion: v, ScannerRevision: stamp.Revision, After: after, BatchSize: remaskBatch,
		})
		if err != nil {
			return 0, fmt.Errorf("reading the subjects stored unmasked under another scanner: %w", err)
		}
		if len(rows) == 0 {
			return total, nil
		}
		stored := make([]pass1core.Stored, 0, len(rows))
		for _, r := range rows {
			stored = append(stored, pass1core.Stored{ID: r.MessageID, Subject: r.Subject})
		}
		after = rows[len(rows)-1].MessageID
		batch := ingest.RemaskStoredSubjectsParams{ScannerVersion: v, ScannerRevision: stamp.Revision}
		masks := maskingrecord.RecordMaskingEventsParams{ScannerVersion: v, ScannerRevision: stamp.Revision}
		for _, m := range remasked(stored) {
			batch.MessageIds = append(batch.MessageIds, m.ID)
			batch.Subjects = append(batch.Subjects, m.Subject)
			batch.SubjectsMasked = append(batch.SubjectsMasked, m.SubjectMasked)
			for _, mask := range m.Masks {
				masks.MessageIds = append(masks.MessageIds, m.ID)
				masks.RuleIds = append(masks.RuleIds, mask.Rule)
				masks.Tiers = append(masks.Tiers, int32(mask.Tier)) //nolint:gosec // A tier is 0, 1 or 2.
			}
		}
		n, err := messages.RemaskStoredSubjects(ctx, batch)
		if err != nil {
			return 0, fmt.Errorf("masking the subjects stored unmasked again: %w", err)
		}
		if n != int64(len(batch.MessageIds)) {
			return 0, fmt.Errorf("masking %d subjects stored unmasked again changed %d rows", len(batch.MessageIds), n)
		}
		if len(masks.MessageIds) > 0 {
			if err := events.RecordMaskingEvents(ctx, masks); err != nil {
				return 0, fmt.Errorf("recording the masking events of the subjects masked again: %w", err)
			}
		}
		total += len(batch.MessageIds)
		if len(rows) < remaskBatch {
			return total, nil
		}
	}
}

// requeueOverturned reads every message the gate skipped with its gate inputs, read as the second
// pass reads the messages waiting for a scan, and returns to pending scan each one overturned names
// (ADR-0121). It returns how many it returned.
func requeueOverturned(ctx context.Context, q *scanstore.Queries, overturned func([]index.Waiting) []string) (int, error) {
	rows, err := q.GateSkips(ctx)
	if err != nil {
		return 0, fmt.Errorf("reading the messages the gate skipped: %w", err)
	}
	skips := make([]index.Waiting, 0, len(rows))
	for _, r := range rows {
		skips = append(skips, index.Waiting{
			ID: r.MessageID, From: r.FromEmail, Domain: r.FromDomain, SubjectMasked: r.SubjectMasked, ListID: r.HasListID,
			SizeBytes: r.SizeBytes, SentAt: mail.UnixMilli(r.SentAt.Time.UnixMilli()), SenderVolume: r.SenderVolume, SenderHits: r.SenderHits,
		})
	}
	ids := overturned(skips)
	if len(ids) == 0 {
		return 0, nil
	}
	n, err := q.RequeueGateSkips(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("returning the overturned gate skips to pending scan: %w", err)
	}
	if n != int64(len(ids)) {
		return 0, fmt.Errorf("returning %d overturned gate skips to pending scan changed %d rows", len(ids), n)
	}
	return len(ids), nil
}
