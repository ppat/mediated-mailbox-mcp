package fake

import (
	"fmt"
	"slices"
	"sync"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters"
)

// Provider is one provider a list of adapters for a test serves, named as an account names it, whose
// accounts connect through an OAuth client when UsesClient is set.
type Provider struct {
	Name       string
	UsesClient bool
}

// Adapters returns a list of adapters for a test, holding one adapter for each of providers, whose
// ceiling is the one a Fake's rate profile declares by default. Its token for an account is a Token
// holding the client and the credential the connections made it from, and its mailbox for an account
// is what mailbox returns for it over that token. A test hands the list to the connections through
// their one option for a test, so a process it runs calls the fake wherever the program calls its
// provider (ADR-0043, ADR-0142). The list is the provider library's own type, a list holding an
// adapter only that library can build, and this package is admitted to test files alone, so nothing
// that ships can select the fake.
func Adapters(mailbox func(account string, token *Token) (*Throttled, error), providers ...Provider) adapters.List {
	byName := make(map[string]adapters.Adapter, len(providers))
	for _, p := range providers {
		byName[p.Name] = adapters.Adapter{
			Facts: adapters.Facts{UsesClient: p.UsesClient, Ceiling: defaultBudget},
			Token: func(client mail.OAuthClient, credential []byte) adapters.TokenSource {
				return &Token{client: client, credential: slices.Clone(credential)}
			},
			Mailbox: func(account string, source adapters.TokenSource, _ adapters.Requests) (adapters.Mailbox, error) {
				token, ok := source.(*Token)
				if !ok {
					return nil, fmt.Errorf("the account's token is a %T, over which the fake builds no mailbox", source)
				}
				return mailbox(account, token)
			},
		}
	}
	return adapters.New(byName)
}

// defaultBudget is the ceiling a Fake's rate profile declares when its configuration names none, which
// New sets.
const defaultBudget = 100

// Token is the fake's token for one account, made from the OAuth client the account connects through
// and its credential. A test plays the provider's part on it, rotating the credential as a refresh
// does and recording an attempt at the provider's authentication endpoint. It is safe for concurrent
// use.
type Token struct {
	mu         sync.Mutex
	client     mail.OAuthClient
	credential []byte
	attempt    mail.AuthAttempt
	atRead     func()
}

// Client returns the OAuth client the token was made with.
func (t *Token) Client() mail.OAuthClient { return t.client }

// Credential returns the credential the token holds now.
func (t *Token) Credential() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.credential)
}

// LastAttempt returns the attempt Attempted recorded last, or the zero value when none was. It first
// calls the function AtAttemptRead set, outside the token's lock.
func (t *Token) LastAttempt() mail.AuthAttempt {
	t.mu.Lock()
	atRead := t.atRead
	t.mu.Unlock()
	if atRead != nil {
		atRead()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.attempt
}

// AtAttemptRead has f called each time the token's latest attempt is read, nil for nothing, so a test
// acts at that point of a unit of work's end, between its read of the token's credential and its
// write-back.
func (t *Token) AtAttemptRead(f func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.atRead = f
}

// Rotate has the token hold credential from now on, as a provider that rotates the credential at a
// refresh does (ADR-0082).
func (t *Token) Rotate(credential []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.credential = slices.Clone(credential)
}

// Attempted records attempt as the token's latest attempt at the provider's authentication endpoint
// (ADR-0097).
func (t *Token) Attempted(attempt mail.AuthAttempt) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempt = attempt
}
