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
)

// Workload is delta sync as job_runs names it (ADR-0016).
const Workload = "sync"

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches.
func NewPostgres(db tx.Beginner) *Postgres { return &Postgres{db: db} }

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// version returns a scanner version as the database stores it.
func version(v int) int32 { return int32(min(max(v, 0), 1<<31-1)) } //nolint:gosec // Bounded.

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string) (State, error) {
	var st State
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		st = State{}
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading the account's progress: %w", err)
		}
		st.SecondEnded = progress.BackfillPass2Complete
		c, err := cursor.New(t).SyncCursor(ctx, account)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading the change cursor: %w", err)
		}
		st.Cursor = mail.Cursor(c.SyncCursor.String)
		if c.SyncCursorAt.Valid {
			at := c.SyncCursorAt.Time.UTC()
			st.CursorAt = &at
		}
		latest, err := runrecord.New(t).LatestRun(ctx, runrecord.LatestRunParams{AccountID: account, Workload: Workload, Pass: PassTick})
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
			if err := endStopped(ctx, q, account); err != nil {
				return err
			}
		}
		err := q.StartRun(ctx, runrecord.StartRunParams{
			AccountID: account, RunID: runID, Workload: Workload, Pass: pass, Checkpoint: cp, Counters: ct,
		})
		if err != nil {
			return fmt.Errorf("recording the run: %w", err)
		}
		return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "start"})
	})
}

// stopped is the last error a run a stopped process left running is recorded with.
const stopped = "the run stopped before it recorded its end"

// endStopped records as failed the account's latest tick and latest gap recovery while either is
// still recorded as running. Ticks do not overlap, so one still running when a tick starts was
// stopped before it recorded its end (ADR-0103).
func endStopped(ctx context.Context, q *runrecord.Queries, account string) error {
	for _, pass := range []string{PassTick, PassGapRecovery} {
		latest, err := q.LatestRun(ctx, runrecord.LatestRunParams{AccountID: account, Workload: Workload, Pass: pass})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading the latest %s run: %w", pass, err)
		}
		if latest.State != "running" {
			continue
		}
		if err := q.EndRun(ctx, runrecord.EndRunParams{State: "failed", LastError: text(stopped), AccountID: account, RunID: latest.RunID}); err != nil {
			return fmt.Errorf("recording the stopped %s run: %w", pass, err)
		}
		if err := q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: latest.RunID, Kind: "failure", Detail: text(stopped)}); err != nil {
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
			domains, err := change.New(t).RemoveMessages(ctx, change.RemoveMessagesParams{AccountID: account, MessageIds: a.Removed})
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
			added, err := insert(ctx, messages, account, m)
			if err != nil {
				return fmt.Errorf("adding message %s: %w", m.ID, err)
			}
			touched[m.Domain] = true
			if added {
				applied.Inserted++
				if m.Unclassified {
					applied.Unclassified++
				}
				if err := recordMasks(ctx, events, account, m); err != nil {
					return err
				}
				continue
			}
			changed, err := relabel(ctx, changes, account, m)
			if err != nil {
				return fmt.Errorf("setting the labels and flags of message %s: %w", m.ID, err)
			}
			if changed {
				applied.Changed++
			}
		}
		for _, d := range slices.Sorted(maps.Keys(touched)) {
			if err := statistics.New(t).RebuildSender(ctx, statistics.RebuildSenderParams{AccountID: account, Domain: d}); err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", d, err)
			}
			if _, err := changes.DropEmptySender(ctx, change.DropEmptySenderParams{AccountID: account, Domain: d}); err != nil {
				return fmt.Errorf("removing the statistics of the sender at %q: %w", d, err)
			}
		}
		if a.Cursor != "" {
			if err := advance(ctx, cursor.New(t), account, a.Cursor); err != nil {
				return err
			}
		}
		return recordProgress(ctx, runrecord.New(t), account, runID, progress(applied))
	})
	return applied, err
}

// advance stores the account's change cursor with its write time.
func advance(ctx context.Context, q *cursor.Queries, account string, c mail.Cursor) error {
	n, err := q.AdvanceCursor(ctx, cursor.AdvanceCursorParams{SyncCursor: pgtype.Text{String: string(c), Valid: true}, AccountID: account})
	if err != nil {
		return fmt.Errorf("storing the change cursor: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("storing the change cursor changed %d rows, want 1", n)
	}
	return nil
}

// insert adds one message to the index and reports whether it was added, false for a message the
// index already held. Its flags and authentication results take the form core/index gives them, the
// form backfill stores too.
func insert(ctx context.Context, q *ingest.Queries, account string, m index.Message) (bool, error) {
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
	_, err = q.InsertMessage(ctx, ingest.InsertMessageParams{
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
	if err != nil {
		return false, err
	}
	return true, media(ctx, q, account, m)
}

// media adds the media of a message's attachments, which the index stores once, with the message
// (ADR-0123).
func media(ctx context.Context, q *ingest.Queries, account string, m index.Message) error {
	if len(m.AttachmentMedia) == 0 {
		return nil
	}
	p := ingest.InsertAttachmentMediaParams{AccountID: account, MessageID: m.ID}
	for _, a := range m.AttachmentMedia {
		p.MediaTypes = append(p.MediaTypes, a.MediaType)
		p.Extensions = append(p.Extensions, a.Extension)
	}
	return q.InsertAttachmentMedia(ctx, p)
}

// relabel sets a stored message's labels and flags to the ones the provider reports, and reports
// whether they changed.
func relabel(ctx context.Context, q *change.Queries, account string, m index.Message) (bool, error) {
	flags, err := json.Marshal(index.StoredFlags(m.Flags))
	if err != nil {
		return false, err
	}
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	n, err := q.ApplyLabelsAndFlags(ctx, change.ApplyLabelsAndFlagsParams{Labels: labels, Flags: flags, AccountID: account, MessageID: m.ID})
	return n == 1, err
}

// recordMasks records a masking event for each mask on m's subject, with the scanner it was masked
// under, as backfill records them (ADR-0003, ADR-0120).
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

// recordProgress records a running run's checkpoint and counters with a progress event, in the
// transaction that made the work they count durable.
func recordProgress(ctx context.Context, q *runrecord.Queries, account, runID string, at Recorder) error {
	cp, ct, err := at.encode()
	if err != nil {
		return err
	}
	if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: cp, Counters: ct, AccountID: account, RunID: runID}); err != nil {
		return fmt.Errorf("recording the run's progress: %w", err)
	}
	return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "progress"})
}

// Since implements Store.
func (s *Postgres) Since(ctx context.Context, account string, start time.Time) ([]string, error) {
	var out []string
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var err error
		out, err = change.New(t).MessagesSince(ctx, change.MessagesSinceParams{AccountID: account, Since: pgtype.Timestamptz{Time: start, Valid: true}})
		return err
	})
	return out, err
}

// Delist implements Store.
func (s *Postgres) Delist(ctx context.Context, account, runID string, delisted func([]string) []string) (int, error) {
	marked := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		marked = 0
		q := scan.New(t)
		restricted, err := q.RestrictedDomains(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the domains stored as restricted or skipped as restricted: %w", err)
		}
		domains := delisted(restricted)
		for _, d := range domains {
			n, err := q.MarkDelisted(ctx, scan.MarkDelistedParams{AccountID: account, Domain: d})
			if err != nil {
				return fmt.Errorf("returning the messages of %q to pending scan: %w", d, err)
			}
			if err := statistics.New(t).RebuildSender(ctx, statistics.RebuildSenderParams{AccountID: account, Domain: d}); err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", d, err)
			}
			marked += int(n)
		}
		if marked == 0 {
			return nil
		}
		return runrecord.New(t).RecordEvent(ctx, runrecord.RecordEventParams{
			AccountID: account, RunID: runID, Kind: "retry",
			Detail: text(fmt.Sprintf("the delisting transition returned %d messages of %d senders the policy no longer restricts to pending scan, "+
				"so the scanning starts from the first message waiting for a scan", marked, len(domains))),
		})
	})
	return marked, err
}

// List implements Store.
func (s *Postgres) List(ctx context.Context, account string, listed func([]string) []index.Listing) (int, error) {
	marked := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		marked = 0
		q := scan.New(t)
		normal, err := q.NormalDomains(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the domains stored as normal: %w", err)
		}
		for _, l := range listed(normal) {
			n, err := q.MarkListed(ctx, scan.MarkListedParams{AccountID: account, Domain: l.Domain, ClassRuleID: text(l.Rule)})
			if err != nil {
				return fmt.Errorf("restricting the stored class of %q: %w", l.Domain, err)
			}
			if err := statistics.New(t).RebuildSender(ctx, statistics.RebuildSenderParams{AccountID: account, Domain: l.Domain}); err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", l.Domain, err)
			}
			marked += int(n)
		}
		return nil
	})
	return marked, err
}

// Pending implements Store.
func (s *Postgres) Pending(ctx context.Context, account, after string, n int) ([]index.Waiting, error) {
	var out []index.Waiting
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		out = nil
		rows, err := scan.New(t).PendingPage(ctx, scan.PendingPageParams{AccountID: account, After: after, PageSize: int32(min(max(n, 1), 1<<31-1))}) //nolint:gosec // Bounded.
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
func (s *Postgres) CommitScan(ctx context.Context, account, runID string, outcomes []index.Outcome, items []Item, at Progress) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		messages, gate := scan.New(t), gaterecord.New(t)
		for _, o := range outcomes {
			if err := commitOutcome(ctx, messages, gate, account, o); err != nil {
				return fmt.Errorf("message %s: %w", o.ID, err)
			}
		}
		hits := index.Hits(outcomes)
		for _, domain := range slices.Sorted(maps.Keys(hits)) {
			n, err := statistics.New(t).AddScanHits(ctx, statistics.AddScanHitsParams{Hits: hits[domain], AccountID: account, Domain: domain})
			if err != nil {
				return fmt.Errorf("adding the prior hits of the sender at %q: %w", domain, err)
			}
			if n != 1 {
				return fmt.Errorf("adding the prior hits of the sender at %q changed %d rows, want 1", domain, n)
			}
		}
		q := runrecord.New(t)
		for _, it := range items {
			err := q.RecordFailure(ctx, runrecord.RecordFailureParams{
				AccountID: account, RunID: runID, ItemKind: "message", ItemID: it.ID, ErrorClass: it.Class,
				ErrorSummary: text(it.Summary), Attempts: 1,
				FirstAt: pgtype.Timestamptz{Time: it.At, Valid: true}, LastAt: pgtype.Timestamptz{Time: it.At, Valid: true},
				Disposition: it.Disposition,
			})
			if err != nil {
				return fmt.Errorf("recording the failed message %s: %w", it.ID, err)
			}
		}
		return recordProgress(ctx, q, account, runID, at)
	})
}

// errNotWaiting is a message a commit found no longer waiting for a scan, which only another process
// writing the same account could cause.
var errNotWaiting = errors.New("the message no longer waits for a scan")

// commitOutcome records what the tick did with one message, as backfill's second pass records it,
// which is the gate's decision when it decided, and the verdict or skip state it leads to. A
// message the gate could not decide, or whose body was not scanned, stays waiting.
func commitOutcome(ctx context.Context, messages *scan.Queries, gate *gaterecord.Queries, account string, o index.Outcome) error {
	if !o.Verdict.Decided() {
		return nil
	}
	decision := "SKIP"
	if o.Verdict.Scans() {
		decision = "SCAN"
	}
	err := gate.RecordDecision(ctx, gaterecord.RecordDecisionParams{AccountID: account, MessageID: o.ID, Decision: decision, Reason: o.Verdict.Reason().String()})
	if err != nil {
		return fmt.Errorf("recording the gate's decision: %w", err)
	}
	var n int64
	switch {
	case o.Scanned != nil:
		flags := []string{}
		if o.Scanned.Flags.MFACode() {
			flags = append(flags, "mfa_code")
		}
		if o.Scanned.Flags.LoginLink() {
			flags = append(flags, "login_link")
		}
		rules := o.Scanned.Rules
		if rules == nil {
			rules = []string{}
		}
		n, err = messages.RecordVerdict(ctx, scan.RecordVerdictParams{
			ContentFlags:    flags,
			RuleIds:         rules,
			ScannerVersion:  pgtype.Int4{Int32: version(o.Scanned.Version), Valid: true},
			ScannerRevision: pgtype.Text{String: o.Scanned.Revision, Valid: true},
			AccountID:       account,
			MessageID:       o.ID,
		})
	case !o.Verdict.Scans():
		n, err = messages.RecordSkip(ctx, scan.RecordSkipParams{ScanState: o.Verdict.State().String(), AccountID: account, MessageID: o.ID})
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if n != 1 {
		return errNotWaiting
	}
	return nil
}

// Backlog implements Store.
func (s *Postgres) Backlog(ctx context.Context, account string) (int64, error) {
	var n int64
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		var err error
		n, err = scan.New(t).Backlog(ctx, account)
		return err
	})
	return n, err
}

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string, c mail.Cursor, at Recorder) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if c != "" {
			if err := advance(ctx, cursor.New(t), account, c); err != nil {
				return err
			}
		}
		q := runrecord.New(t)
		cp, ct, err := at.encode()
		if err != nil {
			return err
		}
		if err := q.RecordProgress(ctx, runrecord.RecordProgressParams{Checkpoint: cp, Counters: ct, AccountID: account, RunID: runID}); err != nil {
			return fmt.Errorf("recording the run's progress: %w", err)
		}
		if err := q.EndRun(ctx, runrecord.EndRunParams{State: "succeeded", AccountID: account, RunID: runID}); err != nil {
			return fmt.Errorf("recording the run's end: %w", err)
		}
		return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "finish"})
	})
}

// Fail implements Store.
func (s *Postgres) Fail(ctx context.Context, account, runID, cause string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		q := runrecord.New(t)
		if err := q.EndRun(ctx, runrecord.EndRunParams{State: "failed", LastError: text(cause), AccountID: account, RunID: runID}); err != nil {
			return err
		}
		return q.RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: "failure", Detail: text(cause)})
	})
}

// Event implements Store.
func (s *Postgres) Event(ctx context.Context, account, runID string, e Event) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return runrecord.New(t).RecordEvent(ctx, runrecord.RecordEventParams{AccountID: account, RunID: runID, Kind: e.Kind, Detail: text(e.Detail)})
	})
}
