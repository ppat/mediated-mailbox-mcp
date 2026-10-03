package fake

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
)

// Consent is the provider fake's Gmail consent, the parts of Google a consent reaches over the
// network held in memory, so the UI's setups are tested over it rather than over Google (ADR-0043).
// It holds the OAuth clients Google recognises and the consents its mailboxes grant, checks a client,
// and exchanges a code for a grant as Google's token endpoint does, refusing a code issued to another
// client, one already used, one whose PKCE verifier does not match its challenge, and a client secret
// that is not the client's. What a consent does without the network, the consent page's address,
// reading the pasted redirect and comparing mailboxes, is the Gmail consent package's own, so the
// fake answers it exactly as the UI's Gmail consent does. It implements mail.Consent and is safe for
// concurrent use.
type Consent struct {
	mu          sync.Mutex
	redirect    string
	clients     map[string]string
	disabled    map[string]bool
	unreachable bool
	pages       map[string]page
	codes       map[string]issued
	seq         int
}

// page is a consent page the UI sent the browser to, keyed by its state.
type page struct {
	clientID  string
	challenge string
}

// issued is an authorization code a consent issued and no exchange has used.
type issued struct {
	clientID  string
	challenge string
	mailbox   string
	scope     string
}

var _ mail.Consent[context.Context] = (*Consent)(nil)

// NewConsent returns a consent that recognises no client and redirects to redirectURI, as the UI's
// configuration names it.
func NewConsent(redirectURI string) *Consent {
	return &Consent{redirect: redirectURI, clients: map[string]string{}, disabled: map[string]bool{}, pages: map[string]page{}, codes: map[string]issued{}}
}

// AddClient makes Google recognise a client, as creating it in the console does.
func (c *Consent) AddClient(id, secret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients[id] = secret
}

// DisableAPI makes the profile read of every grant issued to the client answer that the Gmail API is
// not enabled for the client's project.
func (c *Consent) DisableAPI(clientID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled[clientID] = true
}

// SetUnreachable makes every call that would reach Google fail without an answer, or answer again.
func (c *Consent) SetUnreachable(unreachable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unreachable = unreachable
}

// Grant plays the person at the consent page address grants, signed in as mailbox, granting scope,
// and returns the address Google redirects the browser to. An empty scope grants nothing.
func (c *Consent) Grant(address, mailbox, scope string) (string, error) {
	q, err := consentQuery(address)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pages[q.Get("state")]
	if !ok {
		return "", errors.New("fake: no consent page has this state")
	}
	c.seq++
	code := "fake-code-" + strconv.Itoa(c.seq)
	c.codes[code] = issued{clientID: p.clientID, challenge: p.challenge, mailbox: mailbox, scope: scope}
	return c.redirect + "?" + url.Values{"state": {q.Get("state")}, "code": {code}, "scope": {scope}}.Encode(), nil
}

// Decline plays the person at the consent page address declining, and returns the address Google
// redirects the browser to.
func (c *Consent) Decline(address string) (string, error) {
	q, err := consentQuery(address)
	if err != nil {
		return "", err
	}
	return c.redirect + "?" + url.Values{"state": {q.Get("state")}, "error": {"access_denied"}}.Encode(), nil
}

func consentQuery(address string) (url.Values, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	return u.Query(), nil
}

// CheckClient implements mail.Consent.
func (c *Consent) CheckClient(_ context.Context, client mail.OAuthClient) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unreachable {
		return errors.New("fake: Google did not answer")
	}
	if secret, ok := c.clients[client.ID]; !ok || secret != client.Secret {
		return fmt.Errorf("fake: the client check: %w", mail.ErrClientRefused)
	}
	return nil
}

// ConsentAddress implements mail.Consent, with the Gmail consent's own address, and remembers the
// page by its state.
func (c *Consent) ConsentAddress(clientID, state, verifier, mailbox string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages[state] = page{clientID: clientID, challenge: challenge(verifier)}
	return consent.AuthorizationURL(clientID, c.redirect, state, verifier, mailbox)
}

// ReadRedirect implements mail.Consent, as the Gmail consent reads a pasted address.
func (c *Consent) ReadRedirect(address string) (mail.Redirect, error) {
	return consent.ParseRedirect(address, c.redirect)
}

// Finish implements mail.Consent. A code is used by its first exchange, whatever the answer.
func (c *Consent) Finish(_ context.Context, client mail.OAuthClient, code, verifier string) (mail.Grant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unreachable {
		return mail.Grant{}, errors.New("fake: Google did not answer")
	}
	got, ok := c.codes[code]
	delete(c.codes, code)
	secret, known := c.clients[client.ID]
	switch {
	case !ok, !known, secret != client.Secret, got.clientID != client.ID, got.challenge != challenge(verifier):
		return mail.Grant{}, fmt.Errorf("fake: the code exchange: %w", mail.ErrCodeRefused)
	case got.scope != consent.Scope && got.scope != "" && containsScope(got.scope):
		return mail.Grant{}, fmt.Errorf("fake: the code exchange: %w", mail.ErrScopeRefused)
	case got.scope != consent.Scope:
		return mail.Grant{}, fmt.Errorf("fake: the code exchange: %w", mail.ErrScopeMissing)
	case c.disabled[client.ID]:
		return mail.Grant{}, fmt.Errorf("fake: the profile read: %w", mail.ErrAPIDisabled)
	}
	c.seq++
	return mail.Grant{Credential: "fake-refresh-token-" + strconv.Itoa(c.seq), Mailbox: got.mailbox}, nil
}

// SameMailbox implements mail.Consent, as the Gmail consent compares mailboxes.
func (*Consent) SameMailbox(a, b string) bool { return consent.SameMailbox(a, b) }

// containsScope reports whether a granted scope list holds the modify scope among others.
func containsScope(scope string) bool {
	return slices.Contains(strings.Fields(scope), consent.Scope)
}

// challenge is the S256 PKCE challenge of a verifier.
func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
