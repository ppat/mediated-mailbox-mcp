package provider

import (
	"errors"
	"fmt"
	"net/http"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/google"
	"github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"
)

// Adapters is the list of shipped adapters, each named as an account names its provider, so no
// program names an adapter (ADR-0010). Its representation sits in provider/internal/adapters, which
// the compiler admits to the provider library alone, so no program can build a list holding an
// adapter. Only Shipped and, for tests, the provider fake build one.
type Adapters = adapters.List

// Token is what one account's calls authenticate with, made from the account's credential and, for
// a provider that uses one, the OAuth client the account connects through, through
// Adapters.NewToken (ADR-0106). It reports the credential it holds now, a rotated one once the
// provider rotated it, and its latest attempt, which the connections read at the end of each unit of
// work. It writes nothing (ADR-0082, ADR-0097).
type Token = adapters.Token

// tokenSource and adapterMailbox are the calling side's names for an adapter's token source and the
// mailbox it builds for one account.
type (
	tokenSource    = adapters.TokenSource
	adapterMailbox = adapters.Mailbox
)

// Shipped returns the list of shipped adapters, each sending its requests, its token refreshes
// included, through client.
func Shipped(client *http.Client) Adapters {
	return adapters.New(map[string]adapters.Adapter{
		gmailName: {
			Facts: gmailFacts(),
			Token: func(c coremail.OAuthClient, credential []byte) tokenSource {
				return google.NewTokenSource(client, gmailCredentials(c, credential))
			},
			Mailbox: func(account string, source tokenSource, requests adapters.Requests) (adapterMailbox, error) {
				tokens, ok := source.(gmail.Tokens)
				if !ok {
					return nil, fmt.Errorf("the account's token is a %T, over which the Gmail adapter calls nothing", source)
				}
				return gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: requests})
			},
		},
	})
}

// gmailCredentials returns what a Gmail account's token source is made from, the identifier and
// secret of the OAuth client the account connects through and the account's refresh token, each in
// its own place (ADR-0106).
func gmailCredentials(client coremail.OAuthClient, refreshToken []byte) google.Credentials {
	return google.NewCredentials(client.ID, []byte(client.Secret), refreshToken)
}

// errNoToken is a connection that carries no token.
var errNoToken = errors.New("the connection carries no token")
