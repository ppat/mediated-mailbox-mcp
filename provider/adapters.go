package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/google"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"
)

// Adapters is the list of shipped adapters, each named as an account names its provider, so no
// program names an adapter (ADR-0010).
type Adapters struct {
	byName map[string]adapter
}

// adapter is one shipped adapter, how a token is made for one of its accounts and how its mailbox is
// built over that token.
type adapter struct {
	token   func(client coremail.OAuthClient, credential []byte) tokenSource
	mailbox func(account string, source tokenSource, requests Requests) (adapterMailbox, error)
}

// tokenSource is what an adapter's token is, the credential it holds now and its latest attempt at
// its provider's authentication endpoint.
type tokenSource interface {
	Credential() []byte
	LastAttempt() coremail.AuthAttempt
}

// adapterMailbox is what an adapter builds for one account, the Provider Port's operations and the
// rate profile the adapter declares for them (ADR-0023). The profile is read to lease each call and
// never handed on.
type adapterMailbox interface {
	mail.Mailbox
	RateProfile() coremail.RateLimitProfile[context.Context]
}

// Shipped returns the list of shipped adapters, each sending its requests, its token refreshes
// included, through client.
func Shipped(client *http.Client) Adapters {
	return Adapters{byName: map[string]adapter{
		gmailName: {
			token: func(c coremail.OAuthClient, credential []byte) tokenSource {
				return google.NewTokenSource(client, gmailCredentials(c, credential))
			},
			mailbox: func(account string, source tokenSource, requests Requests) (adapterMailbox, error) {
				tokens, ok := source.(gmail.Tokens)
				if !ok {
					return nil, fmt.Errorf("the account's token is a %T, over which the Gmail adapter calls nothing", source)
				}
				return gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: requests})
			},
		},
	}}
}

// gmailCredentials returns what a Gmail account's token source is made from, the identifier and
// secret of the OAuth client the account connects through and the account's refresh token, each in
// its own place (ADR-0106).
func gmailCredentials(client coremail.OAuthClient, refreshToken []byte) google.Credentials {
	return google.NewCredentials(client.ID, []byte(client.Secret), refreshToken)
}

// Token is what one account's calls authenticate with, made from the account's credential and, for
// a provider that uses one, the OAuth client the account connects through (ADR-0106). It reports the
// credential it holds now, a rotated one once the provider rotated it, and its latest attempt, which
// the connections read at the end of each unit of work. It writes nothing (ADR-0082, ADR-0097).
type Token struct {
	provider string
	source   tokenSource
}

// NewToken makes the token of an account of the provider name, from the OAuth client the account
// connects through, the zero client for a provider that uses none, and the account's credential, as
// plain values the connections opened.
func (a Adapters) NewToken(name string, client coremail.OAuthClient, credential []byte) (*Token, error) {
	entry, ok := a.byName[name]
	if !ok {
		return nil, fmt.Errorf("there is no adapter for provider %q", name)
	}
	return &Token{provider: name, source: entry.token(client, credential)}, nil
}

// Credential returns the credential the token holds now.
func (t *Token) Credential() []byte { return t.source.Credential() }

// LastAttempt returns the token's latest attempt at its provider's authentication endpoint and its
// outcome, or the zero value when it has made none (ADR-0097).
func (t *Token) LastAttempt() coremail.AuthAttempt { return t.source.LastAttempt() }

// errNoToken is a connection that carries no token.
var errNoToken = errors.New("the connection carries no token")
