// Package pass1 holds the decisions of backfill's first pass, as values (ADR-0017, ADR-0040). The
// shell enumerates the mailbox a page at a time and enacts what this package decides.
//
// Begin decides how a run starts from what the index holds, a fresh pass, a resumed one or none at
// all, and Under starts an enumeration made under another scanner over. OnFailure decides what a run
// does after an attempt at a page fails. Decide turns one page of metadata into the rows the index
// stores, each sender classified against the account's policy and each subject masked by the scanner,
// a restricted sender's included (ADR-0003, ADR-0004). Advance moves the checkpoint past a page once
// the page is durable. Unfound masks whole the stored subjects an enumeration that ended did not find
// (ADR-0096).
//
// A checkpoint is the number of pages made durable, the provider's token for the page after them,
// the number of pages the enumeration takes when the provider counts it, and the scanner the
// enumeration masks under. A checkpoint past its first page whose token is empty marks an enumeration
// that has ended, so a run stopped between its last page and its finish only finishes.
package pass1

import (
	"errors"
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
)

// Stamp is the scanner version and configuration revision a subject's masking or a body's scan is
// made under (ADR-0009, ADR-0096).
type Stamp struct {
	Version  int
	Revision string
}

// StampOf returns the stamp of what s decides. A scanner nobody built has a stamp no built one has.
func StampOf(s scan.Scanner) Stamp {
	return Stamp{Version: s.Version(), Revision: s.Revision()}
}

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
	// Stamp is the scanner the enumeration masks under.
	Stamp Stamp
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
	// Remasked counts the messages the index already held whose subject the pass masked again,
	// because it was masked under another scanner (ADR-0096).
	Remasked int
}

// Progress is a pass's checkpoint and counters, as a run records them.
type Progress struct {
	Checkpoint Checkpoint
	Counters   Counters
}

// RunState is a recorded run's state.
type RunState uint8

const (
	// Running is a run still running, or one that stopped without recording its end.
	Running RunState = iota + 1
	// Succeeded is a run that ended its pass.
	Succeeded
	// Failed is a run that ended without ending its pass.
	Failed
)

// Latest is the account's latest recorded run of a pass whose progress is a P.
type Latest[P any] struct {
	// Found is false when no run of the pass was ever recorded.
	Found    bool
	RunID    string
	State    RunState
	Progress P
}

// Start is how a run of a pass whose progress is a P starts.
type Start[P any] struct {
	// Skip is set when the pass has ended for the account and is not due again, so the run does no
	// work.
	Skip bool
	// Reopen is set when the pass had ended and is due again, so the shell records it as not ended
	// as the run starts (ADR-0096).
	Reopen bool
	// ResumedFrom is the run this one resumes, empty for a fresh pass.
	ResumedFrom string
	// Abandon is set when the run resumed is still recorded as running, which it is only when it
	// stopped without recording its end. The shell records it as failed as this run starts.
	Abandon bool
	// From is the progress the run starts from.
	From P
}

// Begin decides how a run of a pass starts for an account whose pass has or has not ended, and is or
// is not due again because the index holds work a scanner other than the one in force decided, from
// its latest recorded run. Both of backfill's passes start this way. A pass that ended and is not due
// is skipped, and one that ended and is due is reopened (ADR-0096). A run that stopped, whether it
// recorded its failure or not, is resumed from its checkpoint and counters. With no run, or when the
// latest one succeeded while the pass is recorded as not ended, which is how a pass is asked to run
// again, a fresh pass starts from the first page.
func Begin[P any](ended, due bool, latest Latest[P]) Start[P] {
	switch {
	case ended && !due:
		return Start[P]{Skip: true}
	case !latest.Found || latest.State == Succeeded:
		return Start[P]{Reopen: ended}
	default:
		return Start[P]{Reopen: ended, ResumedFrom: latest.RunID, Abandon: latest.State == Running, From: latest.Progress}
	}
}

// Under returns how a run of the first pass starts under the scanner stamped s, and whether it starts
// its enumeration over. An enumeration made under another scanner starts over from the first page,
// counters carrying on, so an enumeration that ends was made under one scanner from its first page
// and every message it found was masked under that scanner (ADR-0096).
func Under(start Start[Progress], s Stamp) (Start[Progress], bool) {
	if start.Skip {
		return start, false
	}
	over := start.From.Checkpoint.Stamp != s && start.From.Checkpoint.Page > 0
	if over {
		start.From = Restart(start.From)
	}
	start.From.Checkpoint.Stamp = s
	return start, over
}

// Restart returns the progress of a pass that starts its enumeration over from the first page, as a
// run does when the provider refuses the token it resumed from. The counters and the stamp carry on,
// and the estimate of the pages the enumeration takes is dropped until a page of the new enumeration
// reports one.
func Restart(at Progress) Progress {
	return Progress{Checkpoint: Checkpoint{Stamp: at.Checkpoint.Stamp}, Counters: at.Counters}
}

// Advance returns the progress after one more page is durable, whose next token is next, whose total
// is total, which added added messages to the index and masked the subjects of remasked messages it
// already held again.
func Advance(at Progress, next mail.PageToken, total *mail.Total, added, remasked int) Progress {
	page := at.Checkpoint.Page + 1
	return Progress{
		Checkpoint: Checkpoint{Page: page, Token: next, Of: pagesOf(page, next, total), Stamp: at.Checkpoint.Stamp},
		Counters: Counters{
			Pages:    at.Counters.Pages + 1,
			Messages: at.Counters.Messages + added,
			Remasked: at.Counters.Remasked + remasked,
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

// Class is a sender's class as the index stores it.
type Class string

const (
	// Normal is a sender no rule lists.
	Normal Class = "normal"
	// Restricted is a sender a rule lists, or one that cannot be classified (ADR-0004).
	Restricted Class = "restricted"
)

// Message is one message's row as the index stores it. It holds no body, snippet or attachment name
// (ADR-0016).
type Message struct {
	ID, ThreadID string
	From         mail.Address
	// Domain is the sender's domain in lower case, the part of the address after its last @, or
	// empty for an address without one.
	Domain string
	// Subject is the subject masked at rest (ADR-0003).
	Subject        string
	SubjectMasked  bool
	Date           mail.UnixMilli
	Labels         []string
	Flags          mail.Flags
	HasAttachments bool
	ListID         string
	SizeBytes      int64
	AuthResults    mail.AuthResults
	Class          Class
	// ClassRule is the identifier of the policy rule that set the class, empty when no rule set it,
	// which is a sender no rule lists, a classification made while no policy has loaded and an
	// address that cannot be classified (ADR-0016).
	ClassRule string
	// Unclassified is set when the classifier could not classify the sender, which classifies it
	// restricted.
	Unclassified bool
	// Masks are the masks applied to the subject, one per detection.
	Masks []Mask
	// Stamp is the scanner the subject was masked under (ADR-0096).
	Stamp Stamp
}

// Mask is one mask applied to a subject, naming the rule and tier that detected what was masked and
// never the text (ADR-0003).
type Mask struct {
	Rule string
	Tier int
}

// Page is what one page of the enumeration adds to the index.
type Page struct {
	Messages []Message
	// Domains are the senders' domains the page holds, each once, sorted, whose statistics the shell
	// rebuilds.
	Domains []string
}

// Decide returns the rows one page of metadata adds to the index, each sender classified under the
// account's policy p and each subject masked by s under subject masking's tuning and stamped with s. A
// policy that never loaded restricts every sender, and a scanner nobody built masks every subject
// whole, so neither fails open.
func Decide(items []mail.MessageMetadata, p policy.Composed, s scan.Scanner, l classify.Lookups) Page {
	var out Page
	for _, m := range items {
		verdict := classify.Classify(p, m.From.Email, l)
		masked := redact.MaskSubject(s, m.Subject)
		msg := Message{
			ID:             m.ID,
			ThreadID:       m.ThreadID,
			From:           m.From,
			Domain:         domain(m.From.Email),
			Subject:        masked.Subject(),
			Date:           m.Date,
			Labels:         slices.Clone(m.Labels),
			Flags:          m.Flags,
			HasAttachments: m.HasAttachments,
			ListID:         m.ListID,
			SizeBytes:      m.SizeBytes,
			AuthResults:    m.AuthResults,
			Stamp:          StampOf(s),
			Class:          Normal,
			ClassRule:      verdict.Rule(),
			Unclassified:   verdict.Reason() == classify.Unclassifiable,
		}
		if verdict.Class().Restricted() {
			msg.Class = Restricted
		}
		for _, e := range masked.Events() {
			msg.Masks = append(msg.Masks, Mask{Rule: e.Rule(), Tier: e.Tier()})
		}
		msg.SubjectMasked = len(msg.Masks) > 0
		out.Messages = append(out.Messages, msg)
		out.Domains = append(out.Domains, msg.Domain)
	}
	slices.Sort(out.Domains)
	out.Domains = slices.Compact(out.Domains)
	return out
}

// Stored is a message's subject as the index stores it.
type Stored struct {
	ID, Subject string
}

// Unfound returns the subjects of stored masked whole under the scanner stamped s, as a scanner that
// cannot decide masks a subject, each with the one event of a whole subject. They are the messages an
// enumeration made under s from its first page did not find, so the provider no longer has them and
// their subjects cannot be masked again from what it returns (ADR-0096, ADR-0003).
func Unfound(stored []Stored, s Stamp) []Message {
	var out []Message
	for _, m := range stored {
		masked := redact.MaskSubject(scan.Scanner{}, m.Subject)
		msg := Message{ID: m.ID, Subject: masked.Subject(), Stamp: s}
		for _, e := range masked.Events() {
			msg.Masks = append(msg.Masks, Mask{Rule: e.Rule(), Tier: e.Tier()})
		}
		msg.SubjectMasked = len(msg.Masks) > 0
		out = append(out, msg)
	}
	return out
}

// domain returns the part of address after its last @, lower-cased, or empty when there is none.
func domain(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(address[at+1:])
}

// MaxAttempts is how many times a page the provider throttles or fails is asked for before the run
// fails. The rate limiter paces every attempt, and a later run resumes from the same page.
const MaxAttempts = 5

// ErrorClass is a failure's error class, as a run's failed items record it (ADR-0016).
type ErrorClass string

// The error classes of a failure the provider reported, and NotProvider for one it did not.
const (
	NotProvider    ErrorClass = ""
	Throttled      ErrorClass = "throttled"
	ProviderError  ErrorClass = "provider_error"
	Authentication ErrorClass = "authentication"
	Gone           ErrorClass = "gone"
	Validation     ErrorClass = "validation"
)

// ClassOf returns the error class of a failure the provider reported, by the Provider Port's error it
// wraps, and NotProvider for any other failure, such as the rate limiter failing to record a call.
func ClassOf(err error) ErrorClass {
	switch {
	case errors.Is(err, mail.ErrThrottled):
		return Throttled
	case errors.Is(err, mail.ErrProvider):
		return ProviderError
	case errors.Is(err, mail.ErrAuthentication):
		return Authentication
	case errors.Is(err, mail.ErrNotFound):
		return Gone
	case errors.Is(err, mail.ErrInvalid):
		return Validation
	default:
		return NotProvider
	}
}

// Attempt is what a run knows when an attempt at a page fails.
type Attempt struct {
	Class ErrorClass
	// Attempts counts the attempts at the page, the one that failed included.
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

// OnFailure decides what a run does after an attempt at a page fails. The provider refusing the token
// the run resumed from, which it may no longer honour, starts the enumeration over once, since a page
// taken again counts nothing twice. A throttled or failed page is asked for again until MaxAttempts
// attempts, after a backoff when it was throttled. Every other failure, a refusal of a token the run
// got in this run included, fails the run on the page. A failure that is not the provider's fails the
// run and records no failed page.
func OnFailure(f Attempt) Next {
	switch {
	case f.Class == NotProvider:
		return FailRun
	case f.Class == Validation && f.ResumedToken && !f.Restarted:
		return StartOver
	case f.Class == Throttled && f.Attempts < MaxAttempts:
		return Backoff
	case f.Class == ProviderError && f.Attempts < MaxAttempts:
		return Retry
	default:
		return Abandon
	}
}
