package gmail

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
)

// expiryMargin is how long before an access token's stated expiry it is refreshed, so a request
// never leaves with a token that expires in flight.
const expiryMargin = time.Minute

// Credentials are the OAuth client the account connects through and the account's refresh token, as
// the deployable supplies them from what it opened out of the database (ADR-0080, ADR-0106). Nothing
// in this package reads a credential from anywhere else.
type Credentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// TokenSource hands out access tokens for one account, refreshing them from the refresh token.
//
// When Google rotates the refresh token, the source holds the new one and uses it from then on.
// It also holds its latest request to the token endpoint and that request's outcome. It writes
// nothing. The deployable reads the refresh token and the latest attempt the source holds at the
// end of each unit of work, writes a rotated token back to the account's state row and records
// the attempt there (ADR-0082, ADR-0089, ADR-0097).
type TokenSource struct {
	client *http.Client

	mu      sync.Mutex
	creds   Credentials
	access  string
	expiry  time.Time
	attempt mail.AuthAttempt
}

// NewTokenSource returns a source for the credentials the deployable supplies. Every argument is
// required.
func NewTokenSource(client *http.Client, creds Credentials) *TokenSource {
	return &TokenSource{client: client, creds: creds}
}

// RefreshToken returns the refresh token the source holds, the rotated one once Google has rotated
// it. The deployable compares it with the one it last stored to find a rotation to write back.
func (s *TokenSource) RefreshToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creds.RefreshToken
}

// LastAttempt returns the source's latest request to the token endpoint and its outcome, or the
// zero value when it has made none (ADR-0097). Handing out a held access token is not an attempt.
func (s *TokenSource) LastAttempt() mail.AuthAttempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempt
}

// AccessToken returns an access token valid for at least expiryMargin, refreshing it when needed.
// Every refresh is an attempt the source holds with its outcome, unless the caller cancelled it.
//
// Its composition is tested where a request can fail without a token endpoint, a cached token, a
// cancelled call, a passed deadline and a request that reaches no provider. A refresh Google
// answers needs Google's endpoint, since a stand-in would be a mock (ADR-0043). The contract
// suite's run against real Gmail covers a refresh that succeeds and one Google refuses, and cannot
// make Google rotate a refresh token. So the function stays this thin, and each step keeps its
// logic out of it.
func (s *TokenSource) AccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if token, ok := s.cached(now); ok {
		return token, nil
	}
	body, err := consent.PostForm(ctx, s.client, s.refreshForm())
	if err == nil {
		err = s.receive(body, now)
	}
	s.attempted(ctx, now, err)
	if err != nil {
		return "", err
	}
	return s.access, nil
}

// attempted holds a refresh that started at now and ended with err as the latest attempt, unless
// the caller cancelled it before it succeeded, since a cancelled request has no outcome. A refresh
// whose deadline passed got no answer in time, so it failed (ADR-0097).
func (s *TokenSource) attempted(ctx context.Context, now time.Time, err error) {
	if err != nil && errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	s.attempt = mail.AuthAttempt{At: mail.UnixMilli(now.UnixMilli()), Outcome: consent.Outcome(err)}
}

// cached returns the held access token while it has more than expiryMargin left at now.
func (s *TokenSource) cached(now time.Time) (string, bool) {
	if s.access == "" || !now.Before(s.expiry.Add(-expiryMargin)) {
		return "", false
	}
	return s.access, true
}

// refreshForm is the token request that refreshes the access token, sent with the refresh token
// the source holds.
func (s *TokenSource) refreshForm() url.Values {
	return url.Values{
		"client_id":     {s.creds.ClientID},
		"client_secret": {s.creds.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.creds.RefreshToken},
	}
}

// receive takes the body of a successful refresh response.
func (s *TokenSource) receive(body []byte, now time.Time) error {
	resp, err := consent.ParseTokenResponse(body)
	if err != nil {
		return err
	}
	s.accept(resp, now)
	return nil
}

// accept takes a successful token response, holding the access token and a rotated refresh token.
func (s *TokenSource) accept(resp consent.TokenResponse, now time.Time) {
	s.access, s.expiry = resp.AccessToken, now.Add(resp.ExpiresIn)
	if rotated(s.creds.RefreshToken, resp.RefreshToken) {
		s.creds.RefreshToken = resp.RefreshToken
	}
}

// rotated reports whether a refresh response rotated the refresh token. Google leaves the field
// out when it did not.
func rotated(held, returned string) bool {
	return returned != "" && returned != held
}
