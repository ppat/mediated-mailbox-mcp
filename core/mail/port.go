package mail

import "errors"

// Port is the Provider Port, the one interface every mail backend is reached through (ADR-0010). One
// value serves one account, and every value it returns carries that account's identifier.
//
// C is the value each call carries for cancellation and deadlines, context.Context wherever the port
// is implemented or called. The pure-core import list does not admit the context package, so this
// package names the type only as a parameter.
//
// Every error an operation returns wraps one of the errors below, or is the error C reports when the
// call is cancelled, so a caller never inspects a provider's own error.
type Port[C any] interface {
	// ListThreads returns a page of the threads holding at least one message that q selects, each
	// with every one of its messages. A message in the trash or marked as spam is listed like any
	// other. It returns ErrInvalid for a query Valid refuses.
	ListThreads(ctx C, q Query, page PageToken) (Page[ThreadMetadata], error)
	// GetThreadMetadata returns one thread with every one of its messages, or ErrNotFound.
	GetThreadMetadata(ctx C, threadID string) (ThreadMetadata, error)
	// GetMessageMetadata returns the metadata of each message ids names, in the order of ids, one in
	// the trash or marked as spam included. An identifier naming no message, such as one the mailbox
	// no longer holds, is left out of the result rather than failing the call.
	GetMessageMetadata(ctx C, ids []string) ([]MessageMetadata, error)
	// GetMessageBody returns a message's body, its rendered part as MessageBody describes it, or
	// ErrNotFound. Called with the empty part, it chooses the rendered part, and when the provider
	// holds that part apart from the message it returns a HeldApartError naming it, so the caller
	// names it in a second call, as NextBodyPart decides. Called with a part an earlier call named, it
	// returns that part. It returns ErrRefused, and no body, for a rendered part larger than
	// MaxBodyBytes, decided from the part's declared size before anything is fetched where the
	// provider declares one, or for one it cannot decode (ADR-0131). It is the only operation that
	// returns body content, and the gate guards every call to it (ADR-0002).
	GetMessageBody(ctx C, id string, part BodyPart) (MessageBody, error)
	// ListLabels returns every label the account has, Inbox, Trash and Spam included.
	ListLabels(ctx C) ([]Label, error)
	// EnsureLabel returns the label at path, creating it when the account lacks it. It returns
	// ErrInvalid for an empty path.
	EnsureLabel(ctx C, path string) (Label, error)
	// Mutate applies ops in order and reports each one's outcome. It returns ErrInvalid, and applies
	// nothing, when any op is one no constructor built.
	Mutate(ctx C, ops []MutationOp) (MutationResult, error)
	// CurrentCursor returns the cursor for the mailbox as it is now, the point delta sync first asks
	// for changes from.
	CurrentCursor(ctx C) (Cursor, error)
	// ChangesSince returns the changes after cursor, possibly only some of them. A caller that wants
	// every change asks again from the returned Next until a change set comes back empty. It returns
	// ErrCursorGap when the provider cannot calculate changes from cursor.
	ChangesSince(ctx C, cursor Cursor) (ChangeSet, error)
	// EnumerateAll returns a page of every message in the mailbox. Full traversal is the backfill
	// primitive and never a delta, so delta sync does not call it (ADR-0133, ADR-0018). An
	// implementation that counts the mailbox gives every page its Total, the messages a full
	// enumeration returns as the provider counts them and its page limit, and one that does not
	// gives none (ADR-0095). It returns ErrInvalid for a page token it did not issue.
	EnumerateAll(ctx C, page PageToken) (Page[MessageMetadata], error)
	// RateProfile returns what the implementation's operations cost (ADR-0023).
	RateProfile() RateLimitProfile[C]
}

// The errors the port speaks. An operation's error wraps one of them, so a caller tells them apart
// with errors.Is.
var (
	// ErrNotFound is a message, thread or label the account does not have.
	ErrNotFound = errors.New("mail: no such message, thread or label")
	// ErrInvalid is a request the port refuses as malformed, such as an invalid query, an op no
	// constructor built, or a page token the implementation did not issue.
	ErrInvalid = errors.New("mail: the request is malformed")
	// ErrCursorGap is a cursor the provider cannot calculate changes from, because it expired or was
	// never valid. The caller re-enumerates a bounded window and alerts (ADR-0018).
	ErrCursorGap = errors.New("mail: the provider cannot calculate changes from the cursor")
	// ErrThrottled is a request the provider refused for exceeding its rate. An error wrapping it is
	// a ThrottleError carrying the provider's signal.
	ErrThrottled = errors.New("mail: the provider throttled the request")
	// ErrProvider is a request the provider failed on its side.
	ErrProvider = errors.New("mail: the provider failed the request")
	// ErrAuthentication is a credential the provider refused.
	ErrAuthentication = errors.New("mail: the provider refused the credential")
	// ErrRefused is a body the port will not return, its rendered part larger than MaxBodyBytes or in
	// a form the implementation cannot decode. The provider did nothing wrong, and asking again
	// refuses again while the message and MaxBodyBytes stay as they are (ADR-0131).
	ErrRefused = errors.New("mail: the body is refused, too large or not decodable")
	// ErrHeldApart is a rendered part the provider holds apart from the message, which a second
	// GetMessageBody call names. An error wrapping it is a HeldApartError naming the part.
	ErrHeldApart = errors.New("mail: the body's rendered part is held apart from the message")
)

// BodyPart names one rendered part of a message for GetMessageBody. It is opaque, and only the
// implementation that issued it can read it. The empty part asks the implementation to choose the
// rendered part.
type BodyPart string

// HeldApartError is GetMessageBody's answer when the rendered part is held apart from the message
// and within MaxBodyBytes. Part names it for the second call.
type HeldApartError struct {
	Part BodyPart
}

func (e HeldApartError) Error() string { return ErrHeldApart.Error() }

func (e HeldApartError) Unwrap() error { return ErrHeldApart }

// NextBodyPart decides, after a GetMessageBody call that named asked returned err, the part the next
// call names and whether one is due (ADR-0131). A call that chose the rendered part and was told it
// is held apart is followed by one naming that part. Nothing else is followed, a part that names
// itself held apart again included, so a body takes at most two calls.
func NextBodyPart(asked BodyPart, err error) (BodyPart, bool) {
	var apart HeldApartError
	if asked != "" || !errors.As(err, &apart) || apart.Part == "" {
		return "", false
	}
	return apart.Part, true
}

// MaxBodyBytes is the largest rendered part, in bytes of UTF-8 text, that the port returns and the
// system releases or scans (ADR-0131). The conversion's input limit and the literal form's are this
// value, so the three cannot drift. The live run measured Gmail holding parts apart once a message's
// parts pass between 1,000,000 and 1,048,619 bytes in all, the large parts first, so any raise of
// this value first runs the gmail-contract workflow on its branch, since only a real Gmail answer
// shows how many bodies would then take the second call (ADR-0131).
const MaxBodyBytes = 512 << 10
