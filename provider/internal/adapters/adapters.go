// Package adapters is the representation of the provider library's list of adapters, which the
// calling side, provider, exports as provider.Adapters and fills with the shipped adapters. It sits
// under provider/internal so that the compiler admits it to the provider library alone. The provider
// fake builds a list over itself through it for tests. Code outside the library can name the empty
// list and build no list holding an adapter, so no program can add an adapter or select the fake
// (ADR-0010, ADR-0043, ADR-0142). Its case is argued in provider/README.md.
package adapters

import (
	"context"
	"fmt"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
)

// TokenSource is what an adapter's token is, the credential it holds now and its latest attempt at
// its provider's authentication endpoint.
type TokenSource interface {
	Credential() []byte
	LastAttempt() coremail.AuthAttempt
}

// Mailbox is what an adapter builds for one account, the Provider Port's operations and the rate
// profile the adapter declares for them (ADR-0023). The profile is read to lease each call and never
// handed on.
type Mailbox interface {
	mail.Mailbox
	RateProfile() coremail.RateLimitProfile[context.Context]
}

// Requests are what an adapter reports each request it sends to, the shared provider series as one
// provider sees them.
type Requests interface {
	// Sent adds one request's cost to the account's count.
	Sent(account string, units int)
	// Answered observes one request's latency in seconds under the provider method it called and
	// its outcome.
	Answered(endpoint, outcome string, seconds float64)
}

// Facts are what the rest of the system needs to know about a provider without naming its adapter.
type Facts struct {
	// UsesClient is whether the provider's accounts connect through an OAuth client of the
	// installation (ADR-0106).
	UsesClient bool
	// Ceiling is the provider's declared budget per second, in its own units, which its rate limits
	// derive from (ADR-0023, ADR-0024).
	Ceiling float64
}

// Adapter is one adapter, the facts about its provider, how a token is made for one of its accounts
// and how its mailbox is built over that token.
type Adapter struct {
	Facts   Facts
	Token   func(client coremail.OAuthClient, credential []byte) TokenSource
	Mailbox func(account string, source TokenSource, requests Requests) (Mailbox, error)
}

// List is a list of adapters, each named as an account names its provider. Its zero value holds none.
type List struct {
	byName map[string]Adapter
}

// New returns the list holding each adapter under its name.
func New(byName map[string]Adapter) List {
	held := make(map[string]Adapter, len(byName))
	for name, a := range byName {
		held[name] = a
	}
	return List{byName: held}
}

// Lookup returns the adapter named name in l, and whether l holds one. It is a function rather than
// a method, so only the provider library can call it.
func Lookup(l List, name string) (Adapter, bool) {
	a, ok := l.byName[name]
	return a, ok
}

// Facts returns the facts about the provider name as the list's adapter for it declares them, and
// whether the list holds one.
func (l List) Facts(name string) (Facts, bool) {
	a, ok := l.byName[name]
	return a.Facts, ok
}

// Token is what one account's calls authenticate with, made from the account's credential and, for
// a provider that uses one, the OAuth client the account connects through (ADR-0106). It reports the
// credential it holds now, a rotated one once the provider rotated it, and its latest attempt, which
// the connections read at the end of each unit of work. It writes nothing (ADR-0082, ADR-0097).
type Token struct {
	provider string
	source   TokenSource
}

// NewToken makes the token of an account of the provider name, from the OAuth client the account
// connects through, the zero client for a provider that uses none, and the account's credential, as
// plain values the connections opened.
func (l List) NewToken(name string, client coremail.OAuthClient, credential []byte) (*Token, error) {
	a, ok := l.byName[name]
	if !ok {
		return nil, fmt.Errorf("there is no adapter for provider %q", name)
	}
	return &Token{provider: name, source: a.Token(client, credential)}, nil
}

// Credential returns the credential the token holds now.
func (t *Token) Credential() []byte { return t.source.Credential() }

// LastAttempt returns the token's latest attempt at its provider's authentication endpoint and its
// outcome, or the zero value when it has made none (ADR-0097).
func (t *Token) LastAttempt() coremail.AuthAttempt { return t.source.LastAttempt() }

// ProviderOf returns the provider the token was made for.
func ProviderOf(t *Token) string { return t.provider }

// SourceOf returns the adapter's token source the token holds, which the adapter's mailbox is built
// over.
func SourceOf(t *Token) TokenSource { return t.source }
