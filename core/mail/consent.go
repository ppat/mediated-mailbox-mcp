package mail

import "errors"

// Consent is a provider's OAuth consent, for a provider whose accounts connect through an OAuth
// client of the installation (ADR-0106, ADR-0107). The UI checks a client with it before storing the
// client, and connects or re-authorizes an account through it, from the consent page's address to
// the grant and the mailbox it belongs to. It reads no mailbox beyond naming it, and it is no part of
// the Provider Port.
//
// C is the value each call carries for cancellation and deadlines, as the port's is.
//
// An error a call returns wraps one of the errors below when the provider answered that way. Any
// other error is the provider not answering, or answering in a way the implementation cannot read,
// or the error C reports when the call is cancelled.
type Consent[C any] interface {
	// CheckClient asks the provider whether it recognises the client's identifier and secret. It
	// returns nil for a client the provider accepts and ErrClientRefused for one it refuses.
	CheckClient(ctx C, client OAuthClient) error
	// ConsentAddress returns the consent page's address for the client's identifier, the attempt's
	// state, its PKCE verifier, and the mailbox the provider is asked to offer first.
	ConsentAddress(clientID, state, verifier, mailbox string) string
	// ReadRedirect reads the address the provider sent the browser to after the consent. It returns
	// ErrWrongAddress for an address that is not the consent's redirect address. A redirect that
	// carries an error or no code is returned as read, for the caller to tell apart.
	ReadRedirect(address string) (Redirect, error)
	// Finish exchanges the code for the account's grant, with the client the consent was issued to
	// and the attempt's PKCE verifier, and reads which mailbox granted it. It returns ErrCodeRefused
	// for a code the provider refuses, ErrScopeMissing for a grant without the scope asked for,
	// ErrScopeRefused for one holding any other scope, and ErrAPIDisabled when the mailbox's API is
	// not enabled for the client's project.
	Finish(ctx C, client OAuthClient, code, verifier string) (Grant, error)
	// SameMailbox reports whether two addresses name one mailbox, as the provider identifies a
	// mailbox.
	SameMailbox(a, b string) bool
}

// OAuthClient is an installation's OAuth client as a consent uses it, its identifier and its secret
// in plaintext.
type OAuthClient struct {
	ID     string
	Secret string
}

// Redirect is what the address the provider redirected to carries, the attempt's state, the
// authorization code, and the error the provider names when there is no code.
type Redirect struct {
	State string
	Code  string
	Error string
}

// Grant is a consent's result, the credential the account's workloads authenticate with, which the
// UI seals before it stores it, and the mailbox the provider says granted it.
type Grant struct {
	Credential string
	Mailbox    string
}

// The errors a consent speaks.
var (
	// ErrClientRefused is a client identifier and secret the provider does not recognise.
	ErrClientRefused = errors.New("mail: the provider refused the OAuth client")
	// ErrWrongAddress is a pasted address that is not the consent's redirect address.
	ErrWrongAddress = errors.New("mail: the address is not the consent's redirect address")
	// ErrCodeRefused is an authorization code the provider refused to exchange.
	ErrCodeRefused = errors.New("mail: the provider refused the authorization code")
	// ErrScopeMissing is a grant without the scope the consent asked for.
	ErrScopeMissing = errors.New("mail: the grant lacks the scope asked for")
	// ErrScopeRefused is a grant holding a scope beside the one the consent asked for.
	ErrScopeRefused = errors.New("mail: the grant holds a scope beside the one asked for")
	// ErrAPIDisabled is a mailbox API the client's project has not enabled.
	ErrAPIDisabled = errors.New("mail: the provider's API is not enabled for the client's project")
)
