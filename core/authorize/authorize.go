// Package authorize holds the Mutation Authorizer's matrix. It returns whether a mutation is
// authorized as a verdict value, and the shell enacts it (ADR-0019, ADR-0040).
//
// Sender class governs mutation, and content flags play no part, so a content-flagged message from a
// normal sender keeps every right a normal sender has. A restricted sender's mail may only be
// organized, which is labelling, unlabelling, moving, marking read and starring, and nothing that
// removes a message from view. Permanent delete is not a verb here at all. Which sender class a
// mutation is decided against, and failing a whole batch by authorization class, are the caller's.
//
// Fail closed. The zero Verb is no verb and is refused, as is any value no constant names, and the
// zero Verdict is a refusal.
package authorize

import "github.com/ppat/mediated-mailbox-mcp/core/sensitivity"

// Verb is a mutation the client surface offers. The zero value is no verb.
type Verb uint8

const (
	noVerb Verb = iota
	// Label adds a label.
	Label
	// Unlabel removes a label.
	Unlabel
	// Move moves a message to another folder or label.
	Move
	// MarkRead marks a message read.
	MarkRead
	// Star stars a message.
	Star
	// Archive removes a message from the inbox.
	Archive
	// Trash moves a message to the trash.
	Trash
	// Spam marks a message as spam, which some providers call junk.
	Spam
)

// String names the verb as the client surface spells it.
func (v Verb) String() string {
	switch v {
	case Label:
		return "label"
	case Unlabel:
		return "unlabel"
	case Move:
		return "move"
	case MarkRead:
		return "mark_read"
	case Star:
		return "star"
	case Archive:
		return "archive"
	case Trash:
		return "trash"
	case Spam:
		return "spam"
	case noVerb:
		return "no verb"
	default:
		return "no verb"
	}
}

// Reason says why a mutation was authorized or refused. The zero value is an unknown verb.
type Reason uint8

const (
	// UnknownVerb is a verb no constant names, the zero Verb included.
	UnknownVerb Reason = iota
	// RestrictedSender is a verb that removes a message from view, asked of a restricted sender's
	// mail.
	RestrictedSender
	// Authorized is a mutation the matrix allows.
	Authorized
)

// String names the reason.
func (r Reason) String() string {
	switch r {
	case RestrictedSender:
		return "restricted sender"
	case Authorized:
		return "authorized"
	case UnknownVerb:
		return "unknown verb"
	default:
		return "unknown verb"
	}
}

// Verdict is the authorizer's decision on one mutation. The zero value refuses it as an unknown
// verb.
type Verdict struct {
	reason Reason
}

// Reason returns why the mutation was authorized or refused.
func (v Verdict) Reason() Reason { return v.reason }

// Authorized reports whether the mutation may be applied.
func (v Verdict) Authorized() bool {
	switch v.reason {
	case Authorized:
		return true
	case UnknownVerb, RestrictedSender:
		return false
	default:
		return false
	}
}

// Decide returns the verdict on applying verb to a message from a sender of the given class.
func Decide(verb Verb, class sensitivity.SenderClass) Verdict {
	switch verb {
	case Label, Unlabel, Move, MarkRead, Star:
		return Verdict{reason: Authorized}
	case Archive, Trash, Spam:
		if class.Restricted() {
			return Verdict{reason: RestrictedSender}
		}
		return Verdict{reason: Authorized}
	case noVerb:
		return Verdict{reason: UnknownVerb}
	default:
		return Verdict{reason: UnknownVerb}
	}
}
