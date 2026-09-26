package gmail

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// expiryMargin is how long before an access token's stated expiry it is refreshed, so a request
// never leaves with a token that expires in flight.
const expiryMargin = time.Minute

// Credentials are the installation's OAuth client and the account's refresh token, as the
// deployable supplies them from what it opened out of the database (ADR-0080, ADR-0083). Nothing in
// this package reads a credential from anywhere else.
type Credentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// TokenSource hands out access tokens for one account, refreshing them from the refresh token.
//
// When Google rotates the refresh token, the source holds the new one and uses it from then on.
// It writes nothing. The deployable reads the refresh token the source holds at the end of each
// unit of work and writes a rotated one back to the account's state row (ADR-0082, ADR-0089).
type TokenSource struct {
	client *http.Client

	mu     sync.Mutex
	creds  Credentials
	access string
	expiry time.Time
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

// AccessToken returns an access token valid for at least expiryMargin, refreshing it when needed.
//
// No test covers this function's own composition, the cache check, the post and the hand-over of
// the response body to receive. Each step is tested alone, but running them together needs a
// token endpoint. A stand-in for Google's would be a mock (ADR-0043), and the contract suite's run
// against real Gmail cannot make Google rotate a refresh token. So the function stays this thin,
// and each step keeps its logic out of it.
func (s *TokenSource) AccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if token, ok := s.cached(now); ok {
		return token, nil
	}
	body, err := postForm(ctx, s.client, s.refreshForm())
	if err != nil {
		return "", err
	}
	if err := s.receive(body, now); err != nil {
		return "", err
	}
	return s.access, nil
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
	resp, err := parseTokenResponse(body)
	if err != nil {
		return err
	}
	s.accept(resp, now)
	return nil
}

// accept takes a successful token response, holding the access token and a rotated refresh token.
func (s *TokenSource) accept(resp tokenResponse, now time.Time) {
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
