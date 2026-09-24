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
	// with every one of its messages. It returns ErrInvalid for a query Valid refuses.
	ListThreads(ctx C, q Query, page PageToken) (Page[ThreadMetadata], error)
	// GetThreadMetadata returns one thread with every one of its messages, or ErrNotFound.
	GetThreadMetadata(ctx C, threadID string) (ThreadMetadata, error)
	// GetMessageMetadata returns the metadata of each message ids names, in the order of ids. An
	// identifier naming no message is left out of the result rather than failing the call.
	GetMessageMetadata(ctx C, ids []string) ([]MessageMetadata, error)
	// GetMessageBody returns a message's body, or ErrNotFound. It is the only operation that
	// returns body content, and the gate guards every call to it (ADR-0002).
	GetMessageBody(ctx C, id string) (MessageBody, error)
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
	// primitive and never a delta, so delta sync does not call it (ADR-0017, ADR-0018). It returns
	// ErrInvalid for a page token it did not issue.
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
)
