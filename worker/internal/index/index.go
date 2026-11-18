// Package index writes to the index what both of the worker's job kinds write the same way, once for
// both (ADR-0117): a message added with its attachment media and its masking events, the gate's
// decision on a waiting message with the verdict, refusal or skip it leads to and its sender's prior
// hits, and the delisting transition's enactment (ADR-0003, ADR-0037, ADR-0093, ADR-0094). It also
// holds the loop that decides and scans the messages waiting for a scan, each after the scan of the
// one before it, and the step that renders a body as release serves it and scans that (ADR-0104,
// ADR-0133). The decisions themselves are core/index's, and the counting of what the loop decided is
// worker/internal/index/core's.
//
// Each write takes the Queries of the transaction its job kind opened on its own pool, so what it
// writes becomes durable with the rest of the job kind's page or tick, or not at all, and each job
// kind's list names the subsections it writes through (ADR-0143). Backlog alone reads in a
// transaction of its own. A body is converted and scanned in memory and dropped, so nothing of it
// reaches the store or a log (ADR-0009).
package index

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

	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	coreindex "github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	maskingrecord "github.com/ppat/mediated-mailbox-mcp/db/maskingevents/record"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/ingest"
	scanstore "github.com/ppat/mediated-mailbox-mcp/db/messages/scan"
	gaterecord "github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions/record"
	"github.com/ppat/mediated-mailbox-mcp/db/senders/statistics"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
)

// RefusedBound is the size bound a refusal records and backfill's run-start step compares the stored
// ones with, the port's bound on the rendered part (ADR-0131, ADR-0132).
const RefusedBound = int32(mail.MaxBodyBytes)

// Version returns a scanner version as the database stores it.
func Version(v int) int32 { return int32(min(max(v, 0), 1<<31-1)) } //nolint:gosec // Bounded.

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// Insert adds one message to the index, with its attachment media, in the transaction q runs in, and
// reports whether it was added, false for a message the index already held. Its flags and
// authentication results take the form core/index gives them.
func Insert(ctx context.Context, q *ingest.Queries, m coreindex.Message) (bool, error) {
	flags, err := json.Marshal(coreindex.StoredFlags(m.Flags))
	if err != nil {
		return false, err
	}
	var auth []byte
	if stored := coreindex.StoredAuthResults(m.AuthResults); stored != nil {
		if auth, err = json.Marshal(stored); err != nil {
			return false, err
		}
	}
	labels := m.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err = q.InsertMessage(ctx, ingest.InsertMessageParams{
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
		SubjectScannerVersion:  pgtype.Int4{Int32: Version(m.Stamp.Version), Valid: true},
		SubjectScannerRevision: pgtype.Text{String: m.Stamp.Revision, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, media(ctx, q, m)
}

// media adds the media of a message's attachments, which the index stores once, with the message
// (ADR-0123).
func media(ctx context.Context, q *ingest.Queries, m coreindex.Message) error {
	if len(m.AttachmentMedia) == 0 {
		return nil
	}
	p := ingest.InsertAttachmentMediaParams{MessageID: m.ID}
	for _, a := range m.AttachmentMedia {
		p.MediaTypes = append(p.MediaTypes, a.MediaType)
		p.Extensions = append(p.Extensions, a.Extension)
	}
	return q.InsertAttachmentMedia(ctx, p)
}

// RecordMasks records a masking event for each mask on m's subject, with the scanner it was masked
// under, in the transaction q runs in (ADR-0003, ADR-0120).
func RecordMasks(ctx context.Context, q *maskingrecord.Queries, m coreindex.Message) error {
	for _, mask := range m.Masks {
		err := q.RecordMaskingEvent(ctx, maskingrecord.RecordMaskingEventParams{
			MessageID: m.ID, RuleID: mask.Rule, Tier: int32(mask.Tier), //nolint:gosec // A tier is 0, 1 or 2.
			ScannerVersion:  pgtype.Int4{Int32: Version(m.Stamp.Version), Valid: true},
			ScannerRevision: pgtype.Text{String: m.Stamp.Revision, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("recording a masking event of message %s: %w", m.ID, err)
		}
	}
	return nil
}

// errNotWaiting is a message an outcome's record found no longer waiting for a scan, which only
// another process writing the same account could cause.
var errNotWaiting = errors.New("the message no longer waits for a scan")

// RecordOutcomes records what the job kind did with each of outcomes, in the transaction the Queries
// run in, and adds the flagged verdicts to their senders' prior hits.
func RecordOutcomes(ctx context.Context, messages *scanstore.Queries, gate *gaterecord.Queries, senders *statistics.Queries, outcomes []coreindex.Outcome) error {
	for _, o := range outcomes {
		if err := recordOutcome(ctx, messages, gate, o); err != nil {
			return fmt.Errorf("message %s: %w", o.ID, err)
		}
	}
	hits := coreindex.Hits(outcomes)
	for _, domain := range slices.Sorted(maps.Keys(hits)) {
		n, err := senders.AddScanHits(ctx, statistics.AddScanHitsParams{Hits: hits[domain], Domain: domain})
		if err != nil {
			return fmt.Errorf("adding the prior hits of the sender at %q: %w", domain, err)
		}
		if n != 1 {
			return fmt.Errorf("adding the prior hits of the sender at %q changed %d rows, want 1", domain, n)
		}
	}
	return nil
}

// recordOutcome records what the job kind did with one message, which is the gate's decision when it
// decided, and the verdict, refusal or skip state it leads to. A message the gate could not decide,
// or whose body was not scanned or refused, stays waiting.
func recordOutcome(ctx context.Context, messages *scanstore.Queries, gate *gaterecord.Queries, o coreindex.Outcome) error {
	if !o.Verdict.Decided() {
		return nil
	}
	decision := "SKIP"
	if o.Verdict.Scans() {
		decision = "SCAN"
	}
	err := gate.RecordDecision(ctx, gaterecord.RecordDecisionParams{MessageID: o.ID, Decision: decision, Reason: o.Verdict.Reason().String()})
	if err != nil {
		return fmt.Errorf("recording the gate's decision: %w", err)
	}
	var n int64
	switch {
	case o.Scanned != nil:
		rules := o.Scanned.Rules
		if rules == nil {
			rules = []string{}
		}
		n, err = messages.RecordVerdict(ctx, scanstore.RecordVerdictParams{
			ContentFlags:    o.Scanned.Flags.Names(),
			RuleIds:         rules,
			ScannerVersion:  pgtype.Int4{Int32: Version(o.Scanned.Version), Valid: true},
			ScannerRevision: pgtype.Text{String: o.Scanned.Revision, Valid: true},
			MessageID:       o.ID,
		})
	case o.Refused:
		n, err = messages.RecordRefusal(ctx, scanstore.RecordRefusalParams{RefusedBound: pgtype.Int4{Int32: RefusedBound, Valid: true}, MessageID: o.ID})
	case !o.Verdict.Scans():
		n, err = messages.RecordSkip(ctx, scanstore.RecordSkipParams{ScanState: o.Verdict.State().String(), MessageID: o.ID})
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

// Delist enacts the delisting transition in the transaction q runs in. It reads the domains of the
// messages skipped as restricted and returns those messages of every domain delisted names to pending
// scan (ADR-0037). It returns how many messages it marked and of how many senders, and each job kind
// records what follows from a mark in the same transaction.
func Delist(ctx context.Context, q *scanstore.Queries, delisted func(restricted []string) []string) (marked, senders int, err error) {
	restricted, err := q.RestrictedDomains(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("reading the domains of the messages skipped as restricted: %w", err)
	}
	domains := delisted(restricted)
	for _, d := range domains {
		n, err := q.MarkDelisted(ctx, d)
		if err != nil {
			return 0, 0, fmt.Errorf("returning the messages of %q to pending scan: %w", d, err)
		}
		marked += int(n)
	}
	return marked, len(domains), nil
}

// Backlog returns how many of the account's messages wait for a scan, read in a transaction of its own
// on db.
func Backlog(ctx context.Context, db tx.Beginner, account string) (int64, error) {
	var n int64
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		var err error
		n, err = scanstore.New(t).Backlog(ctx)
		return err
	})
	return n, err
}

// Body is what asking for one message's body came to.
type Body struct {
	// Scanned is what the scan of the body records, nil when it was not scanned.
	Scanned *coreindex.Scanned
	// Refused is set when the port or the conversion refused the body (ADR-0132).
	Refused bool
	// Stop is set when the job kind stops reading at this message, and decides nothing from it.
	Stop bool
}

// GateAndScan decides each of waiting through the gate under the policy p and the thresholds c, with
// the message's age measured at now, and asks body for the body of each the gate selects. Each
// message is decided after the scan of the one before it, so a sender's first hit reaches its next
// message in the same read (ADR-0093). It returns the outcome of each message decided. A Body that
// stops ends the loop at that message and reports stopped, with no outcome of the message it stopped
// on, since the gate would decide the rest without the hits their unscanned bodies hold (ADR-0104).
// An error from body ends the loop with it.
func GateAndScan(p classify.Policy, c scangate.Config, now mail.UnixMilli, waiting []coreindex.Waiting, body func(id string) (Body, error)) (outcomes []coreindex.Outcome, stopped bool, err error) {
	hits := map[string]int64{}
	for _, m := range waiting {
		o := coreindex.Outcome{ID: m.ID, Domain: m.Domain, Verdict: coreindex.Gate(p, c, now, m, hits[m.Domain])}
		if o.Verdict.Scans() {
			b, err := body(m.ID)
			if err != nil {
				return nil, false, err
			}
			if b.Stop {
				return outcomes, true, nil
			}
			o.Refused = b.Refused
			if b.Scanned != nil {
				o.Scanned = b.Scanned
				if b.Scanned.Flags.Any() {
					hits[m.Domain]++
				}
			}
		}
		outcomes = append(outcomes, o)
	}
	return outcomes, false, nil
}

// RenderAndScan renders body as release serves it and scans that with s, timing each step on
// processing, a nil one timing nothing (ADR-0125). It returns the conversion's error when the
// conversion refuses the body, which is then not scanned.
func RenderAndScan(processing *scanner.Processing, s scan.Scanner, body mail.MessageBody) (coreindex.Scanned, error) {
	start := time.Now()
	rendered, err := markdown.Render(body)
	processing.Since(scanner.StepConvert, start)
	if err != nil {
		return coreindex.Scanned{}, err
	}
	start = time.Now()
	scanned := coreindex.Scan(s, rendered)
	processing.Since(scanner.StepScan, start)
	return scanned, nil
}
