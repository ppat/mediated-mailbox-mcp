package mail

import (
	"errors"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
)

// MutationOp is one mutation of one message, named by the verb the Mutation Authorizer judges, so the
// verb a caller authorizes is the verb the port applies (ADR-0019). It is built only through the
// constructors below, and the zero MutationOp is refused by every port.
//
// The label verbs build any label set, Inbox, Trash and Spam included. ADR-0019 lets a restricted
// sender's mail be labelled, unlabelled and moved but not archived, trashed or marked as spam, and
// a label verb can reach the labels those verbs change. What taking a message out of view means
// under ADR-0019 is an open decision gated to M1, which runs the verbs through the Mutation
// Authorizer and whole-batch validation. A constructor sees one op at a time, so it could not hold
// a rule about the state a sequence of ops ends in.
type MutationOp struct {
	verb      authorize.Verb
	messageID string
	add       string
	remove    string
}

// LabelOp returns the op adding the label at path to a message.
func LabelOp(messageID, path string) (MutationOp, error) {
	if err := named(messageID, path); err != nil {
		return MutationOp{}, err
	}
	return MutationOp{verb: authorize.Label, messageID: messageID, add: path}, nil
}

// UnlabelOp returns the op removing the label at path from a message.
func UnlabelOp(messageID, path string) (MutationOp, error) {
	if err := named(messageID, path); err != nil {
		return MutationOp{}, err
	}
	return MutationOp{verb: authorize.Unlabel, messageID: messageID, remove: path}, nil
}

// MoveOp returns the op moving a message from the label at from to the label at to, in one step.
func MoveOp(messageID, from, to string) (MutationOp, error) {
	if err := named(messageID, from); err != nil {
		return MutationOp{}, err
	}
	if err := named(messageID, to); err != nil {
		return MutationOp{}, err
	}
	if from == to {
		return MutationOp{}, errors.Join(ErrInvalid, errors.New("mail: a move needs two different labels"))
	}
	return MutationOp{verb: authorize.Move, messageID: messageID, add: to, remove: from}, nil
}

// ArchiveOp returns the op removing a message from the inbox.
func ArchiveOp(messageID string) (MutationOp, error) { return bare(authorize.Archive, messageID) }

// MarkReadOp returns the op marking a message read.
func MarkReadOp(messageID string) (MutationOp, error) { return bare(authorize.MarkRead, messageID) }

// StarOp returns the op starring a message.
func StarOp(messageID string) (MutationOp, error) { return bare(authorize.Star, messageID) }

// TrashOp returns the op moving a message to the trash, which adds Trash and removes Inbox.
func TrashOp(messageID string) (MutationOp, error) { return bare(authorize.Trash, messageID) }

// SpamOp returns the op marking a message as spam, which adds Spam and removes Inbox.
func SpamOp(messageID string) (MutationOp, error) { return bare(authorize.Spam, messageID) }

// Verb returns the verb the op applies. The zero MutationOp returns the zero Verb, which the
// authorizer refuses.
func (o MutationOp) Verb() authorize.Verb { return o.verb }

// MessageID returns the message the op applies to.
func (o MutationOp) MessageID() string { return o.messageID }

// AddLabel returns the label a Label or Move op adds.
func (o MutationOp) AddLabel() string { return o.add }

// RemoveLabel returns the label an Unlabel or Move op removes.
func (o MutationOp) RemoveLabel() string { return o.remove }

// Built reports whether a constructor built the op. A port refuses a batch holding an op that was
// not.
func (o MutationOp) Built() bool { return o.verb != authorize.Verb(0) && o.messageID != "" }

// MutationResult reports each op's outcome, in the order of the ops. A nil entry is an op the port
// applied, and any other entry wraps one of the port's errors.
type MutationResult struct {
	Errors []error
}

func named(messageID, path string) error {
	if messageID == "" || path == "" {
		return errors.Join(ErrInvalid, errors.New("mail: a label op needs a message and a label"))
	}
	return nil
}

func bare(verb authorize.Verb, messageID string) (MutationOp, error) {
	if messageID == "" {
		return MutationOp{}, errors.Join(ErrInvalid, errors.New("mail: an op needs a message"))
	}
	return MutationOp{verb: verb, messageID: messageID}, nil
}
