// Package pass2 holds the decisions of backfill's second pass, as values (ADR-0017, ADR-0040). The
// shell reads the messages waiting for a scan a page at a time and enacts what this package decides.
//
// Begin decides how a run starts, and only once the first pass has ended for the account. Delisted
// decides which sender domains of the messages stored as restricted or skipped as restricted the
// policy in force no longer restricts, the delisting transition's comparison (ADR-0037). Gate decides whether one message's body is
// scanned, from its own inputs and its sender's current ones (ADR-0093, ADR-0094). Overturned decides
// which stored gate skips a backfill run returns to pending, those the gate no longer decides as the
// same skip (ADR-0098). Scan decides what a scanned body records. OnFailure decides what a run does
// after an attempt at a body fails. Advance moves the checkpoint past a page once the page is durable,
// and Restart starts the pass over.
//
// A checkpoint is the number of pages made durable and the identifier of the last message they read.
// Pages are read in the order of the messages' identifiers, so a message left waiting is read once
// per pass, and an empty page ends the pass.
package pass2

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Checkpoint is where a pass stands.
type Checkpoint struct {
	// Page is how many pages of the pass are durable.
	Page int
	// After is the identifier of the last message the durable pages read, empty before the first.
	After string
}

// Counters count a pass's work across its runs, as ADR-0016 names them.
type Counters struct {
	// Pages counts the pages made durable.
	Pages int
	// Decided counts the messages the gate decided, Scanned those whose scan verdict was recorded,
	// Skipped those recorded as skipped, and Pending those left waiting for a scan, because the gate
	// could not decide or their body could not be fetched or converted.
	Decided, Scanned, Skipped, Pending int
}

// Progress is a pass's checkpoint and counters, as a run records them.
type Progress struct {
	Checkpoint Checkpoint
	Counters   Counters
}

// Begin decides how a run of the pass starts. Until the first pass has ended for the account the run
// is skipped, since the gate reads the statistics the first pass builds and the subjects it masks
// (ADR-0017). After that it starts as the first pass's runs do. It is never found due again by itself,
// since what a change of scanner makes stale for it, and the gate skips the gate no longer decides as
// the same skip, a backfill run's Reopen records before the first pass (ADR-0096, ADR-0098).
func Begin(firstEnded, ended bool, latest pass1.Latest[Progress]) pass1.Start[Progress] {
	if !firstEnded {
		return pass1.Start[Progress]{Skip: true}
	}
	return pass1.Begin(ended, false, latest)
}

// Over returns how a run of the pass starts when a backfill run's start marked the pass to start over,
// and whether it starts over. A run resuming a stopped pass while the mark is set starts over from the
// first message waiting for a scan, counters carrying on, since what that run-start step returned to
// pending may sit before the checkpoint. A fresh pass starts there anyway (ADR-0096).
func Over(start pass1.Start[Progress], marked bool) (pass1.Start[Progress], bool) {
	if start.Skip || !marked || start.ResumedFrom == "" {
		return start, false
	}
	start.From = Restart(start.From)
	return start, true
}

// Restart returns the progress of a pass that starts over from the first message waiting for a scan,
// as a run does when the delisting transition returned messages to pending scan. The counters carry
// on.
func Restart(at Progress) Progress {
	return Progress{Counters: at.Counters}
}

// Delisted returns the domains among restricted, the sender domains of the messages stored as
// restricted or skipped as restricted, that the account policy p no longer restricts, in the order given (ADR-0037). A domain
// the classifier cannot read stays restricted, and a policy that never loaded restricts every domain,
// so neither delists anything.
func Delisted(p policy.Composed, l classify.Lookups, restricted []string) []string {
	var out []string
	for _, d := range restricted {
		if !classify.Classify(p, "@"+d, l).Class().Restricted() {
			out = append(out, d)
		}
	}
	return out
}

// Message is one message waiting for a scan, as the page read returns it, or one the gate skipped, as
// the read of the stored gate skips returns it.
type Message struct {
	ID string
	// From is the sender's address, and Domain its domain as the index stores it.
	From, Domain string
	// SubjectMasked reports whether the first pass masked the subject.
	SubjectMasked bool
	// ListID reports whether the message carries its own List-Id header.
	ListID    bool
	SizeBytes int64
	SentAt    mail.UnixMilli
	// SenderVolume and SenderHits are the sender's volume and prior hits as the page read found them.
	SenderVolume, SenderHits int64
}

// Gate returns the scan gate's decision on m, with the sender's class under the account policy p and
// its prior hits counting pageHits, the hits the run found on this page among the sender's messages
// before m, so a sender's first hit reaches its next message (ADR-0094). The thresholds are c, and
// the message's age is measured at now.
func Gate(p policy.Composed, l classify.Lookups, c scangate.Config, now mail.UnixMilli, m Message, pageHits int64) scangate.Verdict {
	local := m.From
	if at := strings.LastIndexByte(m.From, '@'); at >= 0 {
		local = m.From[:at]
	}
	return scangate.Decide(c, now, scangate.Input{
		Class:         classify.Classify(p, m.From, l).Class(),
		SubjectMasked: m.SubjectMasked,
		ListID:        m.ListID,
		LocalPart:     local,
		SizeBytes:     m.SizeBytes,
		SentAt:        m.SentAt,
		SenderVolume:  m.SenderVolume,
		PriorHits:     m.SenderHits + pageHits,
	})
}

// Overturned returns the identifiers of the stored gate skips among skips that the gate, deciding each
// again, no longer decides as the same skip, in the order given (ADR-0098). It decides as Gate does,
// with the sender's class under the account policy p, the thresholds c and the message's age measured
// at now. A skip stands only while the gate decides it again as high_volume_no_hits, so a message the
// gate would now scan, one under thresholds that cannot decide and one whose sender p now restricts
// are all overturned.
func Overturned(p policy.Composed, l classify.Lookups, c scangate.Config, now mail.UnixMilli, skips []Message) []string {
	var out []string
	for _, m := range skips {
		if Gate(p, l, c, now, m, 0).Reason() != scangate.HighVolumeNoHits {
			out = append(out, m.ID)
		}
	}
	return out
}

// Scanned is what a scanned body records, its content flags, the identifiers of the content rules
// that fired, the scanner version and the configuration's revision (ADR-0009).
type Scanned struct {
	Flags    sensitivity.ContentFlags
	Rules    []string
	Version  int
	Revision string
}

// Scan returns what a body records once scanned by s, from the Markdown of its HTML part and its text
// part. A flag in either flags the message, since release may serve either (ADR-0017). The rules are
// the content rules alone, each once, sorted. A scanner nobody built flags the body, as its verdict
// does.
func Scan(s scan.Scanner, markdown, text string) Scanned {
	a, b := s.Scan(markdown), s.Scan(text)
	rules := append(append([]string{}, a.Rules()...), b.Rules()...)
	slices.Sort(rules)
	return Scanned{
		Flags:    sensitivity.Flags(a.Flags().MFACode() || b.Flags().MFACode(), a.Flags().LoginLink() || b.Flags().LoginLink()),
		Rules:    slices.Compact(rules),
		Version:  a.Version(),
		Revision: a.Revision(),
	}
}

// Attempt is what a run knows when an attempt at a message's body fails.
type Attempt struct {
	Class pass1.ErrorClass
	// Attempts counts the attempts at the body, the one that failed included.
	Attempts int
}

// Next is what a run does after an attempt at a body fails.
type Next uint8

const (
	// Stop records the message as abandoned and fails the run, and a later run resumes from its
	// checkpoint and asks for the body again. The zero value, so a decision nobody made fails the run
	// rather than going on.
	Stop Next = iota
	// Backoff asks for the body again after the throttle's backoff, which the rate limiter waits out.
	Backoff
	// Retry asks for the body again.
	Retry
	// Gone records the message as gone at the provider and goes on. It stays waiting for a scan.
	Gone
	// Abandon records the message as abandoned and goes on. It stays waiting for a scan.
	Abandon
	// FailRun fails the run without recording the message, since the failure is not the message's.
	FailRun
)

// OnFailure decides what a run does after an attempt at a body fails. A throttled body is asked for
// again until pass1.MaxAttempts attempts, after a backoff, and then the run stops, since the provider
// is refusing the run rather than the message. A body the provider fails is asked for again until
// pass1.MaxAttempts attempts and then abandoned, and one it no longer has is gone. Either way the
// message stays waiting and the pass goes on, since a later run would ask for it the same way. A
// refused request is abandoned the same way, and a refused credential stops the run. A failure that
// is not the provider's fails the run.
func OnFailure(f Attempt) Next {
	switch {
	case f.Class == pass1.NotProvider:
		return FailRun
	case f.Class == pass1.Throttled && f.Attempts < pass1.MaxAttempts:
		return Backoff
	case f.Class == pass1.ProviderError && f.Attempts < pass1.MaxAttempts:
		return Retry
	case f.Class == pass1.ProviderError, f.Class == pass1.Validation:
		return Abandon
	case f.Class == pass1.Gone:
		return Gone
	default:
		return Stop
	}
}

// Outcome is what the run did with one message of a page.
type Outcome struct {
	ID, Domain string
	// Verdict is the gate's decision.
	Verdict scangate.Verdict
	// Scanned is what the message's scan records, nil when it was not scanned.
	Scanned *Scanned
}

// Page is what one page of the pass makes durable.
type Page struct {
	Outcomes []Outcome
	// Last is the identifier of the page's last message.
	Last string
}

// Hits returns how many of the page's scanned messages carry a content flag, by the sender's domain.
func (p Page) Hits() map[string]int64 {
	hits := map[string]int64{}
	for _, o := range p.Outcomes {
		if o.Scanned != nil && o.Scanned.Flags.Any() {
			hits[o.Domain]++
		}
	}
	return hits
}

// Advance returns the progress after the page is durable.
func Advance(at Progress, p Page) Progress {
	next := Progress{Checkpoint: Checkpoint{Page: at.Checkpoint.Page + 1, After: p.Last}, Counters: at.Counters}
	next.Counters.Pages++
	for _, o := range p.Outcomes {
		if o.Verdict.Decided() {
			next.Counters.Decided++
		}
		switch {
		case o.Scanned != nil:
			next.Counters.Scanned++
		case o.Verdict.Decided() && !o.Verdict.Scans():
			next.Counters.Skipped++
		default:
			next.Counters.Pending++
		}
	}
	return next
}
