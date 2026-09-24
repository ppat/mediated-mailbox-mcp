package gmail

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// expiryMargin is how long before an access token's stated expiry it is refreshed, so a request
// never leaves with a token that expires in flight.
const expiryMargin = time.Minute

// TokenSource hands out access tokens for one account, refreshing them from the refresh token.
//
// When Google rotates the refresh token, the source keeps the new one in memory and uses it from
// then on, and writes it to the write-back location (ADR-0039). The secret store syncs that
// location, and the refreshed mounted file is what the next start reads. A write that fails is
// logged at error level and tried again after every later refresh until it succeeds, while the
// process goes on with the token it holds.
type TokenSource struct {
	client    *http.Client
	writeBack string
	log       *slog.Logger

	mu    sync.Mutex
	creds Credentials
	// persisted is the refresh token the secret store holds or will receive, the mounted one until
	// a rotated one has been written back.
	persisted string
	access    string
	expiry    time.Time
}

// NewTokenSource returns a source for the credentials Load read, writing a rotated refresh token
// to the file at writeBack. Every argument is required. It first removes any new file an earlier
// process left beside the location when it stopped part of the way through a write-back, once
// that file is older than unfinishedAge.
func NewTokenSource(client *http.Client, creds Credentials, writeBack string, log *slog.Logger) *TokenSource {
	if err := removeUnfinished(writeBack, time.Now()); err != nil {
		log.Error("a new file left beside the Gmail write-back location was not removed, so a refresh token sits outside the location",
			slog.String("location", writeBack), slog.Any("error", err))
	}
	return &TokenSource{
		client:    client,
		writeBack: writeBack,
		log:       log,
		creds:     creds,
		persisted: creds.RefreshToken,
	}
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
// the process holds.
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

// accept takes a successful token response. It holds a rotated refresh token and then writes back
// whatever the store does not yet hold.
func (s *TokenSource) accept(resp tokenResponse, now time.Time) {
	s.access, s.expiry = resp.AccessToken, now.Add(resp.ExpiresIn)
	if rotated(s.creds.RefreshToken, resp.RefreshToken) {
		s.creds.RefreshToken = resp.RefreshToken
	}
	if s.creds.RefreshToken == s.persisted {
		return
	}
	if err := WriteBack(s.writeBack, s.creds.RefreshToken); err != nil {
		s.log.Error("the rotated Gmail refresh token was not written back, so a restart before a later write succeeds loses mailbox access",
			slog.String("location", s.writeBack), slog.Any("error", err))
		return
	}
	s.persisted = s.creds.RefreshToken
}

// rotated reports whether a refresh response rotated the refresh token. Google leaves the field
// out when it did not.
func rotated(held, returned string) bool {
	return returned != "" && returned != held
}
