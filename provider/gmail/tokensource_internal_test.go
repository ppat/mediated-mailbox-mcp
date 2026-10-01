package gmail

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// These tests hand the source the bodies of Google's token responses as literal bytes, at the one
// seam between the HTTP exchange and the source, because the token endpoint is Google's and a
// stand-in for it would test this package's beliefs about it (ADR-0043). What the source would
// send next is read from the request it builds, and what it holds from the refresh token it hands
// the deployable.

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// start builds a source the way a deployable does, from the credentials it opened.
func start(refreshToken string) *TokenSource {
	return NewTokenSource(http.DefaultClient, Credentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RefreshToken: refreshToken,
	})
}

// held is the refresh token the source would send with its next refresh.
func held(src *TokenSource) string {
	return src.refreshForm().Get("refresh_token")
}

// receive hands the source the body of a refresh response.
func receive(t *testing.T, src *TokenSource, body string, at time.Time) {
	t.Helper()
	if err := src.receive([]byte(body), at); err != nil {
		t.Fatalf("receive: %v", err)
	}
}

// Bodies of Google's refresh responses, as its documentation shows them. The first rotates the
// refresh token and the second does not.
const (
	rotatingResponse = `{"access_token": "access", "expires_in": 3599, "refresh_token": "rotated-refresh-token", "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`
	plainResponse    = `{"access_token": "access", "expires_in": 3599, "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`
)

// The adapter's half of VERIFICATIONS' row for forcing a rotation and restarting (ADR-0082). A rotated
// refresh token is held, sent with the next refresh, and handed to the deployable as the source's
// refresh token, which the deployable writes back. A later refresh that rotates nothing keeps it.
func TestARotatedRefreshTokenIsHeldSentAndHandedOver(t *testing.T) {
	src := start("original-refresh-token")

	receive(t, src, rotatingResponse, now)

	if got := held(src); got != "rotated-refresh-token" {
		t.Errorf("the next refresh would send %q, want the rotated token", got)
	}
	if got := src.RefreshToken(); got != "rotated-refresh-token" {
		t.Errorf("the source hands the deployable %q, want the rotated token", got)
	}

	receive(t, src, plainResponse, now.Add(time.Hour))
	if got := src.RefreshToken(); got != "rotated-refresh-token" {
		t.Errorf("after a refresh that rotated nothing the source hands over %q, want the rotated token", got)
	}
}

// A refresh that does not rotate the token leaves the held token as it was. Google leaves the field
// out when it does not rotate, and may return the same token.
func TestARefreshThatDoesNotRotateKeepsTheToken(t *testing.T) {
	for name, body := range map[string]string{
		"field left out": plainResponse,
		"same token":     `{"access_token": "access", "expires_in": 3599, "refresh_token": "original-refresh-token", "token_type": "Bearer"}`,
	} {
		t.Run(name, func(t *testing.T) {
			src := start("original-refresh-token")

			receive(t, src, body, now)

			if got := held(src); got != "original-refresh-token" {
				t.Errorf("the next refresh would send %q, want the original token", got)
			}
			if got := src.RefreshToken(); got != "original-refresh-token" {
				t.Errorf("the source hands over %q, want the original token", got)
			}
		})
	}
}

// An access token is used until a minute before its stated expiry, and refreshed from then on. It
// is never handed over as the refresh token.
func TestAnAccessTokenIsUsedUntilItNearlyExpires(t *testing.T) {
	src := start("original-refresh-token")
	if _, ok := src.cached(now); ok {
		t.Fatal("a source that never refreshed has a cached access token")
	}
	receive(t, src, `{"access_token": "access", "expires_in": 3600, "token_type": "Bearer"}`, now)

	for _, c := range []struct {
		after time.Duration
		want  bool
	}{
		{0, true},
		{58*time.Minute + 59*time.Second, true},
		{59 * time.Minute, false},
		{time.Hour, false},
	} {
		if _, got := src.cached(now.Add(c.after)); got != c.want {
			t.Errorf("cached %v after the refresh = %v, want %v", c.after, got, c.want)
		}
	}
	if got := src.RefreshToken(); got != "original-refresh-token" {
		t.Errorf("the source hands over %q as its refresh token", got)
	}
}

// The refresh request carries the client and the held refresh token.
func TestTheRefreshRequest(t *testing.T) {
	src := start("original-refresh-token")
	want := url.Values{
		"client_id":     {"client-id"},
		"client_secret": {"client-secret"},
		"grant_type":    {"refresh_token"},
		"refresh_token": {"original-refresh-token"},
	}
	if diff := cmp.Diff(want, src.refreshForm(), compare.Options); diff != "" {
		t.Errorf("refresh request (-want +got):\n%s", diff)
	}
}

// unreachable returns a client whose requests fail before they leave the machine, sent through a
// proxy on a port nothing listens on. The failure is the real transport's, with no stand-in for
// Google answering anything (ADR-0043).
func unreachable() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "127.0.0.1:9"})}}
}

// A refresh that gets no answer is an attempt that failed, held with the instant it started, so the
// deployable records it as transient rather than as a refused credential (ADR-0097).
func TestARefreshThatGetsNoAnswerIsAFailedAttempt(t *testing.T) {
	src := NewTokenSource(unreachable(), Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"})
	before := time.Now().UnixMilli()

	if _, err := src.AccessToken(t.Context()); err == nil {
		t.Fatal("a refresh through an unreachable proxy returned an access token")
	}

	after := time.Now().UnixMilli()
	got := src.LastAttempt()
	if got.Outcome != "failed" {
		t.Errorf("the attempt's outcome is %q, want failed", got.Outcome)
	}
	if int64(got.At) < before || int64(got.At) > after {
		t.Errorf("the attempt is held at %d, want an instant from %d to %d", got.At, before, after)
	}
}

// A refresh its caller cancelled has no outcome, so the source holds no attempt for it (ADR-0097).
func TestACancelledRefreshIsNoAttempt(t *testing.T) {
	src := NewTokenSource(unreachable(), Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := src.AccessToken(ctx); err == nil {
		t.Fatal("a cancelled refresh returned an access token")
	}

	if got := src.LastAttempt(); got != (mail.AuthAttempt{}) {
		t.Errorf("after a cancelled refresh the source holds the attempt %+v, want none", got)
	}
}

// A refresh whose deadline passed before an answer arrived got no answer, so it is an attempt that
// failed, not a cancellation (ADR-0097).
func TestARefreshPastItsDeadlineIsAFailedAttempt(t *testing.T) {
	src := NewTokenSource(unreachable(), Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"})
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	if _, err := src.AccessToken(ctx); err == nil {
		t.Fatal("a refresh past its deadline returned an access token")
	}

	if got := src.LastAttempt().Outcome; got != "failed" {
		t.Errorf("a refresh past its deadline is held as %q, want failed", got)
	}
}

// Handing out an access token the source holds is not an attempt, so a source whose calls all run
// on a held token reports none (ADR-0097).
func TestAHeldAccessTokenIsNoAttempt(t *testing.T) {
	src := NewTokenSource(unreachable(), Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"})
	receive(t, src, plainResponse, time.Now())

	token, err := src.AccessToken(t.Context())
	if err != nil || token != "access" {
		t.Fatalf("AccessToken returned %q, %v, want the held token", token, err)
	}

	if got := src.LastAttempt(); got != (mail.AuthAttempt{}) {
		t.Errorf("handing out the held token left the attempt %+v, want none", got)
	}
}
