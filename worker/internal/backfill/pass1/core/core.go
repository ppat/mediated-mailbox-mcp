// Package core holds the decisions of backfill's first pass, as values (ADR-0133, ADR-0040). The
// shell enumerates the mailbox a page at a time and enacts what this package decides.
//
// How a run starts from what the index holds, a fresh pass, a resumed one or none at all, is
// worker/internal/runs/core's Begin, which both passes share. Under carries the checkpoint of an
// enumeration that ended into a pass reopened after it and moves every starting checkpoint to the scanner in force with the stale subjects it has to fetch, so
// an enumeration not yet ended continues whatever scanner it was made under. OnFailure decides what a
// run does after an attempt at a page, or at a call fetching subjects again, fails. What a page adds to
// the index is core/index's Decide, which delta sync shares (ADR-0003, ADR-0004). Advance moves the
// checkpoint past a page once the page is durable.
//
// Whenever stored subjects are masked under another scanner, the pass fetches them again by
// identifier, a call at a time, before it enumerates further, so a run fetches them before it
// continues an enumeration not yet ended and once the enumeration has ended. Refetch masks each again from the subject the provider
// returns and masks whole, through Unfound, the subject of each message the provider left out of its
// answer, since the provider no longer has it, and Fetched moves the progress past a call once it is
// durable. Remasked masks again from the store a subject stored unmasked, which backfill's run-start
// step does for every stale one before the first pass (ADR-0120).
//
// A checkpoint is the number of pages made durable, the provider's token for the page after them,
// the number of pages the enumeration takes when the provider counts it, the scanner the run masks
// under, and the stored subjects still to fetch again. A checkpoint past its first page whose token
// is empty marks an enumeration that has ended, so a run stopped between its last page and its finish
// only fetches what is stale and finishes.
package core

import (
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	runs "github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core"
)

// Checkpoint is where a pass stands.
type Checkpoint struct {
	// Page is how many pages of the enumeration are durable.
	Page int
	// Token is the provider's token for the next page, empty before the first page and after the
	// last.
	Token mail.PageToken
	// Of is how many pages the enumeration takes, an estimate from the total the provider reported
	// with the page made durable, and zero when it reported none (ADR-0095).
	Of int
	// Stamp is the scanner the run masks under.
	Stamp index.Stamp
	// Stale is how many stored subjects were masked under another scanner than Stamp when the
	// checkpoint was written, the subjects the pass has still to fetch again before it enumerates
	// further or ends. A run's start, each call fetching subjects again and the page that ends the
	// enumeration record it, and every other page records zero (ADR-0120).
	Stale int
}

// Ended reports whether the enumeration the checkpoint follows has ended.
func (c Checkpoint) Ended() bool { return c.Page > 0 && c.Token == "" }

// Counters count a pass's work across its runs.
type Counters struct {
	// Pages counts the pages made durable, a page taken again after the pass started over included.
	Pages int
	// Messages counts the messages the pass added to the index. A message the index already held is
	// not counted again.
	Messages int
	// Remasked counts the messages the index already held whose subject a page of the enumeration
	// masked again, because it was masked under another scanner (ADR-0120).
	Remasked int
	// Refetched counts the stored subjects masked under another scanner that the pass fetched again
	// by identifier and masked again, a subject masked whole because the provider no longer has its
	// message included (ADR-0120).
	Refetched int
}

// Progress is a pass's checkpoint and counters, as a run records them.
type Progress struct {
	Checkpoint Checkpoint
	Counters   Counters
}

// Under returns how a run of the first pass starts under the scanner stamped s, from the start Begin
// decided and the latest recorded run, while stale stored subjects are masked under another scanner.
// A pass reopened after its enumeration ended starts from that enumeration's checkpoint with fresh
// counters, so it enumerates nothing and only fetches the stale subjects again. A run resuming an
// enumeration not yet ended continues it from its checkpoint, its page, token, estimate and counters,
// whatever scanner it was made under. The subjects its earlier pages stored under another scanner are
// stale like any other: the run-start step has masked the unmasked ones again from the store, and the
// masked ones are the stale subjects the run fetches again before it continues. Every run that starts
// records s and the stale subjects it has to fetch (ADR-0120).
func Under(start runs.Start[Progress], latest runs.Latest[Progress], s index.Stamp, stale int) runs.Start[Progress] {
	if start.Skip {
		return start
	}
	if start.Reopen && start.ResumedFrom == "" && latest.Found && latest.Progress.Checkpoint.Ended() {
		ended := latest.Progress.Checkpoint
		start.From = Progress{Checkpoint: Checkpoint{Page: ended.Page, Of: ended.Of}}
	}
	start.From.Checkpoint.Stamp = s
	start.From.Checkpoint.Stale = stale
	return start
}

// Restart returns the progress of a pass that starts its enumeration over from the first page, as a
// run does when the provider refuses the token it resumed from. The counters and the stamp carry on,
// and the estimate of the pages the enumeration takes is dropped until a page of the new enumeration
// reports one.
func Restart(at Progress) Progress {
	return Progress{Checkpoint: Checkpoint{Stamp: at.Checkpoint.Stamp}, Counters: at.Counters}
}

// Fetched returns the progress after one more call fetching stale subjects again is durable, which
// masked fetched subjects again and left stale subjects masked under another scanner (ADR-0120).
func Fetched(at Progress, fetched, stale int) Progress {
	at.Checkpoint.Stale = stale
	at.Counters.Refetched += fetched
	return at
}

// Advance returns the progress after one more page is durable, whose next token is next, whose total
// is total, which added added messages to the index and masked the subjects of remasked messages it
// already held again.
func Advance(at Progress, next mail.PageToken, total *mail.Total, added, remasked int) Progress {
	page := at.Checkpoint.Page + 1
	return Progress{
		Checkpoint: Checkpoint{Page: page, Token: next, Of: pagesOf(page, next, total), Stamp: at.Checkpoint.Stamp},
		Counters: Counters{
			Pages:     at.Counters.Pages + 1,
			Messages:  at.Counters.Messages + added,
			Remasked:  at.Counters.Remasked + remasked,
			Refetched: at.Counters.Refetched,
		},
	}
}

// pagesOf is how many pages an enumeration takes, judged at the page numbered page, whose next token
// is next and whose total is total, from those alone (ADR-0095). With no total it is zero, for no
// estimate. At the last page it is that page. Before it, it is the total's items over its page
// limit, rounded up, and never fewer than one page past this one, since a next token means another
// page.
func pagesOf(page int, next mail.PageToken, total *mail.Total) int {
	switch {
	case total == nil || total.PageLimit <= 0:
		return 0
	case next == "":
		return page
	default:
		return max(page+1, (max(total.Items, 0)+total.PageLimit-1)/total.PageLimit)
	}
}

// Stored is a message's subject as the index stores it.
type Stored struct {
	ID, Subject string
}

// Unfound returns the subjects of stored masked whole under the scanner stamped s, as a scanner that
// cannot decide masks a subject, each with the one event of a whole subject. They are the messages the
// provider left out of its answer to a fetch by identifier, so it no longer has them and their
// subjects cannot be masked again from what it returns (ADR-0120, ADR-0003).
func Unfound(stored []Stored, s index.Stamp) []index.Message {
	var out []index.Message
	for _, m := range stored {
		msg := masked(m.ID, m.Subject, scan.Scanner{})
		msg.Stamp = s
		out = append(out, msg)
	}
	return out
}

// Remasked returns the subjects of stored, each stored unmasked and so the subject the provider
// returned, masked again by s, with an event for each mask (ADR-0120). A subject s masks nothing of
// is returned as stored.
func Remasked(stored []Stored, s scan.Scanner) []index.Message {
	out := make([]index.Message, 0, len(stored))
	for _, m := range stored {
		out = append(out, masked(m.ID, m.Subject, s))
	}
	return out
}

// Refetch returns the stored subjects the pass asked the provider for again, split into those masked
// again by s from the subject the provider returned and those masked whole under s's stamp because the
// provider left their message out of its answer, as the Provider Port's metadata read does for a
// message the mailbox no longer holds. Metadata for a message not asked for is ignored (ADR-0120).
func Refetch(stored []Stored, got []mail.MessageMetadata, s scan.Scanner) (remasked, gone []index.Message) {
	returned := make(map[string]string, len(got))
	for _, m := range got {
		returned[m.ID] = m.Subject
	}
	var unfound []Stored
	for _, m := range stored {
		subject, ok := returned[m.ID]
		if !ok {
			unfound = append(unfound, m)
			continue
		}
		remasked = append(remasked, masked(m.ID, subject, s))
	}
	return remasked, Unfound(unfound, index.StampOf(s))
}

// masked returns the message id's subject masked by s, stamped with s, with an event for each mask.
func masked(id, subject string, s scan.Scanner) index.Message {
	m := redact.MaskSubject(s, subject)
	msg := index.Message{ID: id, Subject: m.Subject(), Stamp: index.StampOf(s)}
	for _, e := range m.Events() {
		msg.Masks = append(msg.Masks, index.Mask{Rule: e.Rule(), Tier: e.Tier()})
	}
	msg.SubjectMasked = len(msg.Masks) > 0
	return msg
}

// MaxAttempts is how many times a page or a call the provider throttles or fails is asked for before
// the run fails. The rate limiter paces every attempt, and a later run resumes from the same page or
// asks for the same stale subjects.
const MaxAttempts = 5

// Attempt is what a run knows when an attempt at a page or a call fails.
type Attempt struct {
	Class index.ErrorClass
	// Attempts counts the attempts at the page or call, the one that failed included.
	Attempts int
	// ResumedToken is set when the token the page was asked with is the one the run resumed from,
	// before the run made any page durable.
	ResumedToken bool
	// Restarted is set when the run already started its enumeration over.
	Restarted bool
}

// Next is what a run does after an attempt at a page fails.
type Next uint8

const (
	// Abandon fails the run on the page, and a later run resumes from its checkpoint. The zero value,
	// so a decision nobody made fails the run rather than retrying without end.
	Abandon Next = iota
	// Backoff asks for the page again after the throttle's backoff, which the rate limiter waits out.
	Backoff
	// Retry asks for the page again.
	Retry
	// StartOver starts the enumeration over from the first page, as Restart moves the progress.
	StartOver
	// FailRun fails the run without recording a failed page, since the failure is not the page's,
	// and a later run resumes from its checkpoint.
	FailRun
)

// OnFailure decides what a run does after an attempt at a page, or at a call fetching stale subjects
// again, fails. The provider refusing the token the run resumed from, which it may no longer honour,
// starts the enumeration over once, since a page taken again counts nothing twice. A throttled or
// failed page or call is asked for again until MaxAttempts attempts, after a backoff when it was
// throttled. Every other failure, a refusal of a token the run got in this run included, fails the
// run on the page or call. A failure that is not the provider's fails the run and records no failed
// item.
func OnFailure(f Attempt) Next {
	switch {
	case f.Class == index.NotProvider:
		return FailRun
	case f.Class == index.Validation && f.ResumedToken && !f.Restarted:
		return StartOver
	case f.Class == index.Throttled && f.Attempts < MaxAttempts:
		return Backoff
	case f.Class == index.ProviderError && f.Attempts < MaxAttempts:
		return Retry
	default:
		return Abandon
	}
}
