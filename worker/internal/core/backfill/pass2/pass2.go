// Package pass2 holds the decisions of backfill's second pass, as values (ADR-0017, ADR-0040). The
// shell reads the messages waiting for a scan a page at a time and enacts what this package decides.
//
// Begin decides how a run starts, and only once the first pass has ended for the account. The delisting
// transition's comparison, the gate and what a scanned body records are core/index's, which delta sync
// shares (ADR-0037, ADR-0093, ADR-0094). Overturned decides which stored gate skips backfill's
// run-start step returns to pending, those the gate no longer decides as the same skip (ADR-0121).
// OnFailure decides what a run does after an attempt at a body fails. Advance moves the checkpoint
// past a page once the page is durable, and Restart starts the pass over.
//
// A checkpoint is the number of pages made durable and the identifier of the last message they read.
// Pages are read in the order of the messages' identifiers, so a message left waiting is read once
// per pass, and an empty page ends the pass.
package pass2

import (
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
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
// the same skip, the run-start step's Reopen records before the first pass (ADR-0120, ADR-0121).
func Begin(firstEnded, ended bool, latest pass1.Latest[Progress]) pass1.Start[Progress] {
	if !firstEnded {
		return pass1.Start[Progress]{Skip: true}
	}
	return pass1.Begin(ended, false, latest)
}

// Over returns how a run of the pass starts when backfill's run-start step marked the pass to start
// over, and whether it starts over. A run resuming a stopped pass while the mark is set starts over
// from the first message waiting for a scan, counters carrying on, since what that run-start step
// returned to pending may sit before the checkpoint. A fresh pass starts there anyway (ADR-0120).
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

// Overturned returns the identifiers of the stored gate skips among skips that the gate, deciding each
// again, no longer decides as the same skip, in the order given (ADR-0121). It decides as Gate does,
// with the sender's class under the account policy p, the thresholds c and the message's age measured
// at now. A skip stands only while the gate decides it again as high_volume_no_hits, so a message the
// gate would now scan, one under thresholds that cannot decide and one whose sender p now restricts
// are all overturned.
func Overturned(p policy.Composed, l classify.Lookups, c scangate.Config, now mail.UnixMilli, skips []index.Waiting) []string {
	var out []string
	for _, m := range skips {
		if index.Gate(p, l, c, now, m, 0).Reason() != scangate.HighVolumeNoHits {
			out = append(out, m.ID)
		}
	}
	return out
}

// Attempt is what a run knows when an attempt at a message's body fails.
type Attempt struct {
	Class index.ErrorClass
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
	case f.Class == index.NotProvider:
		return FailRun
	case f.Class == index.Throttled && f.Attempts < pass1.MaxAttempts:
		return Backoff
	case f.Class == index.ProviderError && f.Attempts < pass1.MaxAttempts:
		return Retry
	case f.Class == index.ProviderError, f.Class == index.Validation:
		return Abandon
	case f.Class == index.Gone:
		return Gone
	default:
		return Stop
	}
}

// Page is what one page of the pass makes durable.
type Page struct {
	Outcomes []index.Outcome
	// Last is the identifier of the page's last message.
	Last string
}

// Hits returns how many of the page's scanned messages carry a content flag, by the sender's domain.
func (p Page) Hits() map[string]int64 { return index.Hits(p.Outcomes) }

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
