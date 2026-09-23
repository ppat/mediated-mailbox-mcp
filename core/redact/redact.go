// Package redact holds the Redaction Gate's decision and subject masking's detection (ADR-0003). The
// decision returns a verdict naming which fields of a message a client receives, and the shell
// enacts it (ADR-0001, ADR-0002, ADR-0040).
//
// The gate decides from the content flags and scan state the index stores and from the policy in
// force when it is asked, never from anything a client supplies. It classifies the sender again
// against that policy and never reads the sender class the index stored, so a sender listed after its
// message was synced is denied on the next call. The decision takes no provider and no body, so a
// denied body is never fetched and the verdict has nowhere to hold one.
//
// The field-level matrix is ADR-0001's. Sender, thread, date, labels, flags and subject are always
// shown, and the subject is served as stored because subject masking masks it at rest (ADR-0003).
// The snippet and attachment filenames are derived from the body, so they follow it. A client
// receives them with a released body, and otherwise no snippet and only the attachment types.
//
// Fail closed. Content flags on a message the scanner has not read are a stored state no message can
// be in, and the gate denies it before anything else, as invalid rather than as pending. The zero
// Verdict is that denial.
package redact

import (
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Reason says why the gate released or withheld a body. The zero value is an invalid stored state.
type Reason uint8

const (
	// InvalidState is a stored state no message can be in, content flags on a message the scanner has
	// not read.
	InvalidState Reason = iota
	// PendingScan is a message the scanner has not reached yet.
	PendingScan
	// SkippedRestricted is a message left unscanned because its sender was restricted when it was
	// synced.
	SkippedRestricted
	// RestrictedSender is a sender the policy in force restricts, including one the classifier cannot
	// read and every sender while no policy has loaded.
	RestrictedSender
	// ContentFlagged is a message in which the scanner found a one-time code or a login link.
	ContentFlagged
	// Released is a body the gate releases.
	Released
)

// String names the reason.
func (r Reason) String() string {
	switch r {
	case PendingScan:
		return "pending content scan"
	case SkippedRestricted:
		return "skipped as restricted"
	case RestrictedSender:
		return "restricted sender"
	case ContentFlagged:
		return "content flagged"
	case Released:
		return "released"
	case InvalidState:
		return "invalid stored state"
	default:
		return "invalid stored state"
	}
}

// Verdict is the gate's decision on one message. The zero value withholds the body as an invalid
// stored state.
type Verdict struct {
	reason Reason
	sender classify.Verdict
}

// Reason returns why the body was released or withheld.
func (v Verdict) Reason() Reason { return v.reason }

// Sender returns the sender's classification against the policy in force. A shell building the
// sensitivity a released body is constructed with takes the sender class from here, never from the
// class the index stored.
func (v Verdict) Sender() classify.Verdict { return v.sender }

// ReleasesBody reports whether the client receives the body.
func (v Verdict) ReleasesBody() bool {
	switch v.reason {
	case Released:
		return true
	case InvalidState, PendingScan, SkippedRestricted, RestrictedSender, ContentFlagged:
		return false
	default:
		return false
	}
}

// ShowsSnippet reports whether the client receives the snippet. It follows the body.
func (v Verdict) ShowsSnippet() bool { return v.ReleasesBody() }

// ShowsAttachmentNames reports whether the client receives attachment filenames rather than only
// their types. They follow the body.
func (v Verdict) ShowsAttachmentNames() bool { return v.ReleasesBody() }

// Decide returns the gate's verdict on a message from sender, with the content flags and scan state
// the index stores, under the account policy p. The lookups are the classifier's.
func Decide(p policy.Composed, sender string, l classify.Lookups, flags sensitivity.ContentFlags, scan sensitivity.ScanState) Verdict {
	c := classify.Classify(p, sender, l)
	v := Verdict{sender: c}
	switch {
	case flags.Any() && scan != sensitivity.Scanned():
		v.reason = InvalidState
	case scan == sensitivity.Pending():
		v.reason = PendingScan
	case scan == sensitivity.SkippedRestricted():
		v.reason = SkippedRestricted
	case c.Class().Restricted():
		v.reason = RestrictedSender
	case flags.Any():
		v.reason = ContentFlagged
	case scan == sensitivity.Scanned(), scan == sensitivity.SkippedGate():
		v.reason = Released
	default:
		v.reason = InvalidState
	}
	return v
}
