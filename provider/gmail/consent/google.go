package consent

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Google is Gmail's consent as the UI runs it, redirecting to the loopback address the UI's
// configuration names, where nothing listens. The browser lands on a page it cannot reach, and the
// operator pastes its address back (ADR-0107, docs/UI.md section 8.12).
type Google struct {
	client   *http.Client
	redirect string
}

var _ mail.Consent[context.Context] = Google{}

// New returns the consent, sending its requests through client and redirecting to redirectURI,
// which every request and every reading of a pasted address names.
func New(client *http.Client, redirectURI string) Google {
	return Google{client: client, redirect: redirectURI}
}

// CheckClient implements mail.Consent.
func (g Google) CheckClient(ctx context.Context, client mail.OAuthClient) error {
	return CheckClient(ctx, g.client, client.ID, client.Secret, g.redirect)
}

// ConsentAddress implements mail.Consent. The mailbox is Google's login hint.
func (g Google) ConsentAddress(clientID, state, verifier, mailbox string) string {
	return AuthorizationURL(clientID, g.redirect, state, verifier, mailbox)
}

// ReadRedirect implements mail.Consent.
func (g Google) ReadRedirect(address string) (mail.Redirect, error) {
	return ParseRedirect(address, g.redirect)
}

// Finish implements mail.Consent. The grant's credential is the refresh token, and its mailbox is
// the profile's address read with the first access token. A code exchange the token endpoint refuses,
// a 400 or a 401, is mail.ErrCodeRefused.
func (g Google) Finish(ctx context.Context, client mail.OAuthClient, code, verifier string) (mail.Grant, error) {
	grant, err := ExchangeCode(ctx, g.client, client.ID, client.Secret, code, verifier, g.redirect)
	if err != nil {
		if Outcome(err) == mail.AuthRefused {
			return mail.Grant{}, fmt.Errorf("%w: %w", mail.ErrCodeRefused, err)
		}
		return mail.Grant{}, err
	}
	address, err := Address(ctx, g.client, grant.AccessToken)
	if err != nil {
		return mail.Grant{}, err
	}
	return mail.Grant{Credential: grant.RefreshToken, Mailbox: address}, nil
}

// SameMailbox implements mail.Consent.
func (Google) SameMailbox(a, b string) bool { return SameMailbox(a, b) }
