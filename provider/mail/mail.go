// Package mail is what any mail provider can do, the Provider Port for mail, Mailbox, with the call
// size a caller needs, and the consent contract a provider whose accounts connect through an OAuth
// client implements (ADR-0010, ADR-0106, ADR-0107). The canonical model the port speaks, its query,
// its mutations, its errors, the part of a body a call names, the decision on a second body call and
// the types a rate profile is stated in are core/mail's.
package mail

import (
	"context"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Mailbox is the Provider Port for mail, one account's mailbox (ADR-0010). Every value it returns
// carries the account's identifier. Every error an operation returns wraps one of core/mail's port
// errors, or is the context's error when the call is cancelled, so a caller never inspects a
// provider's own error.
//
// What an operation costs is the adapter's declaration and no part of the port. The provider library
// reads it to lease each call, so code that calls a Mailbox reads the call size it carries and no
// cost (ADR-0023).
type Mailbox interface {
	// ListThreads returns a page of the threads holding at least one message that q selects, each
	// with every one of its messages. A message in the trash or marked as spam is listed like any
	// other. It returns ErrInvalid for a query Valid refuses.
	ListThreads(ctx context.Context, q coremail.Query, page coremail.PageToken) (coremail.Page[coremail.ThreadMetadata], error)
	// GetThreadMetadata returns one thread with every one of its messages, or ErrNotFound.
	GetThreadMetadata(ctx context.Context, threadID string) (coremail.ThreadMetadata, error)
	// GetMessageMetadata returns the metadata of each message ids names, in the order of ids, one in
	// the trash or marked as spam included. An identifier naming no message, such as one the mailbox
	// no longer holds, is left out of the result rather than failing the call. A caller names no more
	// identifiers in one call than MetadataCallSize gives.
	GetMessageMetadata(ctx context.Context, ids []string) ([]coremail.MessageMetadata, error)
	// GetMessageBody returns a message's body, its rendered part as MessageBody describes it, or
	// ErrNotFound. Called with the empty part, it chooses the rendered part, and when the provider
	// holds that part apart from the message it returns a HeldApartError naming it, so the caller
	// names it in a second call, as NextBodyPart decides. Called with a part an earlier call named, it
	// returns that part. It returns ErrRefused, and no body, for a rendered part larger than
	// MaxBodyBytes, decided from the part's declared size before anything is fetched where the
	// provider declares one, or for one it cannot decode (ADR-0131). It is the only operation that
	// returns body content, and the gate guards every call to it (ADR-0002).
	GetMessageBody(ctx context.Context, id string, part coremail.BodyPart) (coremail.MessageBody, error)
	// ListLabels returns every label the account has, Inbox, Trash and Spam included.
	ListLabels(ctx context.Context) ([]coremail.Label, error)
	// EnsureLabel returns the label at path, creating it when the account lacks it. It returns
	// ErrInvalid for an empty path.
	EnsureLabel(ctx context.Context, path string) (coremail.Label, error)
	// Mutate applies ops in order and reports each one's outcome. It returns ErrInvalid, and applies
	// nothing, when any op is one no constructor built.
	Mutate(ctx context.Context, ops []coremail.MutationOp) (coremail.MutationResult, error)
	// CurrentCursor returns the cursor for the mailbox as it is now, the point delta sync first asks
	// for changes from.
	CurrentCursor(ctx context.Context) (coremail.Cursor, error)
	// ChangesSince returns the changes after cursor, possibly only some of them. A caller that wants
	// every change asks again from the returned Next until a change set comes back empty. It returns
	// ErrCursorGap when the provider cannot calculate changes from cursor.
	ChangesSince(ctx context.Context, cursor coremail.Cursor) (coremail.ChangeSet, error)
	// EnumerateAll returns a page of every message in the mailbox. Full traversal is the backfill
	// primitive and never a delta, so delta sync does not call it (ADR-0133, ADR-0018). An
	// implementation that counts the mailbox gives every page its Total, the messages a full
	// enumeration returns as the provider counts them and its page limit, and one that does not
	// gives none (ADR-0095). It returns ErrInvalid for a page token it did not issue.
	EnumerateAll(ctx context.Context, page coremail.PageToken) (coremail.Page[coremail.MessageMetadata], error)

	// MetadataCallSize returns how many identifiers one GetMessageMetadata call names, out of most
	// to ask for. It is what core/mail's CallSize gives for the implementation's rate profile, the
	// most whose cost fits one second's worth at the hard cap and at least one, so no call is refused
	// when its lease is issued (ADR-0023, ADR-0024).
	MetadataCallSize(most int) int
}

// Consent is a provider's OAuth consent, for a provider whose accounts connect through an OAuth
// client of the installation (ADR-0106, ADR-0107). The UI checks a client with it before storing the
// client, and connects or re-authorizes an account through it, from the consent page's address to
// the grant and the mailbox it belongs to. It reads no mailbox beyond naming it, and it is no part of
// the Provider Port.
//
// An error a call returns wraps one of core/mail's consent errors when the provider answered that
// way. Any other error is the provider not answering, or answering in a way the implementation cannot
// read, or the context's error when the call is cancelled.
type Consent interface {
	// CheckClient asks the provider whether it recognises the client's identifier and secret. It
	// returns nil for a client the provider accepts and ErrClientRefused for one it refuses.
	CheckClient(ctx context.Context, client coremail.OAuthClient) error
	// ConsentAddress returns the consent page's address for the client's identifier, the attempt's
	// state, its PKCE verifier, and the mailbox the provider is asked to offer first.
	ConsentAddress(clientID, state, verifier, mailbox string) string
	// ReadRedirect reads the address the provider sent the browser to after the consent. It returns
	// ErrWrongAddress for an address that is not the consent's redirect address. A redirect that
	// carries an error or no code is returned as read, for the caller to tell apart.
	ReadRedirect(address string) (coremail.Redirect, error)
	// Finish exchanges the code for the account's grant, with the client the consent was issued to
	// and the attempt's PKCE verifier, and reads which mailbox granted it. It returns ErrCodeRefused
	// for a code the provider refuses, ErrScopeMissing for a grant without the scope asked for,
	// ErrScopeRefused for one holding any other scope, and ErrAPIDisabled when the mailbox's API is
	// not enabled for the client's project.
	Finish(ctx context.Context, client coremail.OAuthClient, code, verifier string) (coremail.Grant, error)
	// SameMailbox reports whether two addresses name one mailbox, as the provider identifies a
	// mailbox.
	SameMailbox(a, b string) bool
}

// MetadataCallSize returns what core/mail's CallSize gives for a GetMessageMetadata call under
// profile, out of most identifiers to ask for, which is what an implementation's MetadataCallSize
// returns.
func MetadataCallSize(profile coremail.RateLimitProfile[context.Context], most int) int {
	return coremail.CallSize(func(n int) float64 {
		return profile.Cost(coremail.ProviderOp{Operation: coremail.OpGetMessageMetadata, Messages: n}).Weight
	}, profile.BudgetPerSecond(), most)
}
