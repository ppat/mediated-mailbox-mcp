package pass2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	pass1core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/scan"
	decisions "github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions/record"
	"github.com/ppat/mediated-mailbox-mcp/db/senders"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// pass is the pass as job_runs names it (ADR-0016).
const pass = "pass2"

// Postgres is the Store in the index's database. Each method runs one transaction through db/tx,
// which sets the account.
type Postgres struct {
	db tx.Beginner
}

var _ Store = (*Postgres)(nil)

// NewPostgres returns the Store in the database db reaches.
func NewPostgres(db tx.Beginner) *Postgres { return &Postgres{db: db} }

// checkpoint and counters are the stored forms of a run's checkpoint and counters. The checkpoint's
// page is the page the jobs card shows, and after is the identifier of the last message the durable
// pages read.
type checkpoint struct {
	Page  int    `json:"page"`
	After string `json:"after"`
}

type counters struct {
	Pages   int `json:"pages"`
	Decided int `json:"decided"`
	Pending int `json:"pending"`
	Scanned int `json:"scanned"`
	Skipped int `json:"skipped"`
}

func encode(p core.Progress) (cp, ct []byte, err error) {
	cp, err = json.Marshal(checkpoint{Page: p.Checkpoint.Page, After: p.Checkpoint.After})
	if err != nil {
		return nil, nil, err
	}
	c := p.Counters
	ct, err = json.Marshal(counters{Pages: c.Pages, Decided: c.Decided, Pending: c.Pending, Scanned: c.Scanned, Skipped: c.Skipped})
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
		Checkpoint: core.Checkpoint{Page: c.Page, After: c.After},
		Counters:   core.Counters{Pages: n.Pages, Decided: n.Decided, Pending: n.Pending, Scanned: n.Scanned, Skipped: n.Skipped},
	}, nil
}

// version returns a scanner version as the database stores it.
func version(v int) int32 { return int32(min(max(v, 0), 1<<31-1)) } //nolint:gosec // Bounded.

// State implements Store.
func (s *Postgres) State(ctx context.Context, account string) (bool, bool, bool, pass1core.Latest[core.Progress], error) {
	var (
		first, ended, restart bool
		latest                pass1core.Latest[core.Progress]
	)
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		if err != nil {
			return fmt.Errorf("reading the completion flags: %w", err)
		}
		first, ended = progress.BackfillPass1Complete, progress.BackfillPass2Complete
		if restart, err = completion.New(t).SecondRestart(ctx, account); err != nil {
			return fmt.Errorf("reading whether the pass is marked to start over: %w", err)
		}
		r, err := pass1.LatestRun(ctx, record.New(t), account, pass)
		if err != nil || !r.Found {
			return err
		}
		latest = pass1core.Latest[core.Progress]{Found: true, RunID: r.RunID, State: r.State}
		latest.Progress, err = decode(r.Checkpoint, r.Counters)
		return err
	})
	return first, ended, restart, latest, err
}

// Start implements Store.
func (s *Postgres) Start(ctx context.Context, account, runID string, start pass1core.Start[core.Progress]) error {
	cp, ct, err := encode(start.From)
	if err != nil {
		return err
	}
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := completion.New(t).ClearSecondRestart(ctx, account); err != nil {
			return fmt.Errorf("clearing the mark that starts the pass over: %w", err)
		}
		return pass1.StartRun(ctx, record.New(t), account, pass1.Starting{
			Pass: pass, RunID: runID, ResumedFrom: start.ResumedFrom, Abandon: start.Abandon,
			Checkpoint: cp, Counters: ct, Page: start.From.Checkpoint.Page,
		})
	})
}

// Delist implements Store.
func (s *Postgres) Delist(ctx context.Context, account, runID string, delisted func([]string) []string, restart core.Progress) (int, error) {
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
			if err := senders.New(t).RebuildSender(ctx, senders.RebuildSenderParams{AccountID: account, Domain: d}); err != nil {
				return fmt.Errorf("rebuilding the statistics of the sender at %q: %w", d, err)
			}
			marked += int(n)
		}
		if marked == 0 {
			return nil
		}
		cp, ct, err := encode(restart)
		if err != nil {
			return err
		}
		r := record.New(t)
		if err := pass1.RecordProgress(ctx, r, account, runID, cp, ct, restart.Checkpoint.Page); err != nil {
			return err
		}
		return pass1.RecordEvent(ctx, r, account, runID, pass1.Event{
			Kind: "retry",
			Detail: fmt.Sprintf("the delisting transition returned %d messages of %d senders the policy no longer restricts to pending scan, "+
				"so the pass starts over from the first message waiting for a scan", marked, len(domains)),
		})
	})
	return marked, err
}

// Reopen implements Store.
func (s *Postgres) Reopen(ctx context.Context, account string, stamp index.Stamp, overturned func([]index.Waiting) []string) (Reopened, error) {
	var marked Reopened
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		marked = Reopened{}
		domains, err := scan.New(t).RequeueStaleVerdicts(ctx, scan.RequeueStaleVerdictsParams{
			AccountID: account, ScannerVersion: version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("returning the verdicts another scanner made to pending scan: %w", err)
		}
		slices.Sort(domains)
		for _, d := range slices.Compact(slices.Clone(domains)) {
			n, err := senders.New(t).RecountScanHits(ctx, senders.RecountScanHitsParams{AccountID: account, Domain: d})
			if err != nil {
				return fmt.Errorf("counting the prior hits of the sender at %q again: %w", d, err)
			}
			if n != 1 {
				return fmt.Errorf("counting the prior hits of the sender at %q again changed %d rows, want 1", d, n)
			}
		}
		skips, err := requeueOverturned(ctx, scan.New(t), account, overturned)
		if err != nil {
			return err
		}
		stale, err := ingest.New(t).StaleSubject(ctx, ingest.StaleSubjectParams{
			AccountID: account, ScannerVersion: version(stamp.Version), ScannerRevision: stamp.Revision,
		})
		if err != nil {
			return fmt.Errorf("reading whether a subject was masked under another scanner: %w", err)
		}
		marked = Reopened{Verdicts: len(domains), Skips: skips}
		if marked.Verdicts == 0 && marked.Skips == 0 && !stale {
			return nil
		}
		if err := completion.New(t).ReopenBackfillSecond(ctx, account); err != nil {
			return fmt.Errorf("recording the pass as due again: %w", err)
		}
		return nil
	})
	return marked, err
}

// requeueOverturned reads every message the gate skipped with its gate inputs, read as Pending reads
// them, and returns to pending scan each one overturned names (ADR-0098). It returns how many it
// returned.
func requeueOverturned(ctx context.Context, q *scan.Queries, account string, overturned func([]index.Waiting) []string) (int, error) {
	rows, err := q.GateSkips(ctx, account)
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
	n, err := q.RequeueGateSkips(ctx, scan.RequeueGateSkipsParams{AccountID: account, MessageIds: ids})
	if err != nil {
		return 0, fmt.Errorf("returning the overturned gate skips to pending scan: %w", err)
	}
	if n != int64(len(ids)) {
		return 0, fmt.Errorf("returning %d overturned gate skips to pending scan changed %d rows", len(ids), n)
	}
	return len(ids), nil
}

// RequeueSkips implements Store.
func (s *Postgres) RequeueSkips(ctx context.Context, account, runID string) (int, error) {
	marked := 0
	err := tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		skips, err := scan.New(t).RequeueSignalledSkips(ctx, account)
		if err != nil {
			return fmt.Errorf("returning the skips decided without their subject's signal to pending scan: %w", err)
		}
		marked = int(skips)
		if marked == 0 {
			return nil
		}
		return pass1.RecordEvent(ctx, record.New(t), account, runID, pass1.Event{
			Kind:   "retry",
			Detail: fmt.Sprintf("%d skips decided without their subject's signal returned to pending scan", skips),
		})
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

// errNotWaiting is a message a page's commit found no longer waiting for a scan, which only another
// process writing the same account could cause.
var errNotWaiting = errors.New("the message no longer waits for a scan")

// Commit implements Store.
func (s *Postgres) Commit(ctx context.Context, account, runID string, p core.Page, items []pass1.Item, at core.Progress) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		messages, gate := scan.New(t), decisions.New(t)
		for _, o := range p.Outcomes {
			if err := commitOutcome(ctx, messages, gate, account, o); err != nil {
				return fmt.Errorf("message %s: %w", o.ID, err)
			}
		}
		hits := p.Hits()
		for _, domain := range slices.Sorted(maps.Keys(hits)) {
			n, err := senders.New(t).AddScanHits(ctx, senders.AddScanHitsParams{Hits: hits[domain], AccountID: account, Domain: domain})
			if err != nil {
				return fmt.Errorf("adding the prior hits of the sender at %q: %w", domain, err)
			}
			if n != 1 {
				return fmt.Errorf("adding the prior hits of the sender at %q changed %d rows, want 1", domain, n)
			}
		}
		r := record.New(t)
		for _, it := range items {
			if err := pass1.RecordItem(ctx, r, account, runID, it); err != nil {
				return fmt.Errorf("recording the failed message %s: %w", it.ID, err)
			}
		}
		cp, ct, err := encode(at)
		if err != nil {
			return err
		}
		return pass1.RecordProgress(ctx, r, account, runID, cp, ct, at.Checkpoint.Page)
	})
}

// commitOutcome records what the run did with one message: the gate's decision when it decided, and
// the verdict or skip state it leads to. A message the gate could not decide, or whose body was not
// scanned, stays waiting.
func commitOutcome(ctx context.Context, messages *scan.Queries, gate *decisions.Queries, account string, o index.Outcome) error {
	if !o.Verdict.Decided() {
		return nil
	}
	decision := "SKIP"
	if o.Verdict.Scans() {
		decision = "SCAN"
	}
	err := gate.RecordDecision(ctx, decisions.RecordDecisionParams{AccountID: account, MessageID: o.ID, Decision: decision, Reason: o.Verdict.Reason().String()})
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
			ScannerVersion:  pgtype.Int4{Int32: int32(min(max(o.Scanned.Version, 0), 1<<31-1)), Valid: true}, //nolint:gosec // Bounded.
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

// Finish implements Store.
func (s *Postgres) Finish(ctx context.Context, account, runID string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		if err := completion.New(t).SetBackfillSecondComplete(ctx, account); err != nil {
			return fmt.Errorf("setting the completion flag: %w", err)
		}
		return pass1.SucceedRun(ctx, record.New(t), account, runID)
	})
}

// Fail implements Store.
func (s *Postgres) Fail(ctx context.Context, account, runID, cause string) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return pass1.FailRun(ctx, record.New(t), account, runID, cause)
	})
}

// Event implements Store.
func (s *Postgres) Event(ctx context.Context, account, runID string, e pass1.Event) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return pass1.RecordEvent(ctx, record.New(t), account, runID, e)
	})
}

// Failure implements Store.
func (s *Postgres) Failure(ctx context.Context, account, runID string, it pass1.Item) error {
	return tx.Run(ctx, s.db, account, func(t pgx.Tx) error {
		return pass1.RecordItem(ctx, record.New(t), account, runID, it)
	})
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
