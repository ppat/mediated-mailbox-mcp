package gmail

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Token endpoint bodies in the shape Google documents, read as literal bytes.
func TestParseTokenResponse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want tokenResponse
	}{
		{
			"refresh that rotates",
			`{"access_token": "ya29.access", "expires_in": 3599, "refresh_token": "1//rotated", "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`,
			tokenResponse{AccessToken: "ya29.access", ExpiresIn: 3599 * time.Second, RefreshToken: "1//rotated", Scope: "https://www.googleapis.com/auth/gmail.modify"},
		},
		{
			"refresh that does not rotate",
			`{"access_token": "ya29.access", "expires_in": 3599, "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`,
			tokenResponse{AccessToken: "ya29.access", ExpiresIn: 3599 * time.Second, Scope: "https://www.googleapis.com/auth/gmail.modify"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseTokenResponse([]byte(c.body))
			if err != nil {
				t.Fatalf("parseTokenResponse: %v", err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("parseTokenResponse (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseTokenResponseRefusesAnUnusableBody(t *testing.T) {
	for name, body := range map[string]string{ //nolint:gosec // test values, not credentials
		"not JSON":          `access_token=ya29.access`,
		"no access token":   `{"expires_in": 3599, "refresh_token": "1//rotated"}`,
		"no lifetime":       `{"access_token": "ya29.access"}`,
		"negative lifetime": `{"access_token": "ya29.access", "expires_in": -1}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := parseTokenResponse([]byte(body)); err == nil {
				t.Errorf("parseTokenResponse accepted %s as %+v", body, got)
			}
		})
	}
}

// The code exchange carries the PKCE verifier and the redirect address the consent used, which
// Google checks against the consent page's request.
func TestTheCodeExchangeRequest(t *testing.T) {
	want := url.Values{
		"client_id":     {"client-id"},
		"client_secret": {"client-secret"},
		"code":          {"the-code"},
		"code_verifier": {"verifier"},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {"http://127.0.0.1:8080/"},
	}
	got := exchangeForm("client-id", "client-secret", "the-code", "verifier", "http://127.0.0.1:8080/")
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("code exchange request (-want +got):\n%s", diff)
	}
}

// The code exchange accepts a grant of the modify scope alone. A grant holding any other scope,
// the full mail scope that can permanently delete among them, or holding none, is refused, so the
// token is known to lack permanent delete (ADR-0011).
func TestTheExchangeAcceptsOnlyTheModifyScope(t *testing.T) {
	if err := onlyModify("https://www.googleapis.com/auth/gmail.modify"); err != nil {
		t.Errorf("onlyModify refused the modify scope alone: %v", err)
	}
	for name, scope := range map[string]string{
		"no scope":                     "",
		"the full mail scope":          "https://mail.google.com/",
		"modify and the full scope":    "https://www.googleapis.com/auth/gmail.modify https://mail.google.com/",
		"modify and a settings scope":  "https://www.googleapis.com/auth/gmail.settings.basic https://www.googleapis.com/auth/gmail.modify",
		"modify twice":                 "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/gmail.modify",
		"read-only in place of modify": "https://www.googleapis.com/auth/gmail.readonly",
	} {
		if err := onlyModify(scope); err == nil {
			t.Errorf("onlyModify accepted %s", name)
		}
	}
}

// The profile names the account a token belongs to.
func TestParseProfileAddress(t *testing.T) {
	got, err := parseProfileAddress([]byte(`{"emailAddress": "test@example.com", "messagesTotal": 3, "threadsTotal": 2, "historyId": "12"}`))
	if err != nil || got != "test@example.com" {
		t.Errorf("parseProfileAddress = %q, %v, want test@example.com", got, err)
	}
	for _, body := range []string{`{"historyId": "12"}`, `not JSON`} {
		if got, err := parseProfileAddress([]byte(body)); err == nil {
			t.Errorf("parseProfileAddress accepted %s as %q", body, got)
		}
	}
}

// The code exchange's response is a grant only when it holds the modify scope alone and a refresh
// token, read from bodies in the shape Google documents.
func TestGrantFrom(t *testing.T) {
	got, err := grantFrom([]byte(`{"access_token": "ya29.access", "expires_in": 3599, "refresh_token": "1//grant", "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`))
	if err != nil {
		t.Fatalf("grantFrom refused a grant of the modify scope alone: %v", err)
	}
	if diff := cmp.Diff(Grant{RefreshToken: "1//grant", AccessToken: "ya29.access"}, got, compare.Options); diff != "" {
		t.Errorf("grantFrom (-want +got):\n%s", diff)
	}
	for name, body := range map[string]string{ //nolint:gosec // test values, not credentials
		"the full mail scope":       `{"access_token": "ya29.access", "expires_in": 3599, "refresh_token": "1//grant", "scope": "https://mail.google.com/"}`,
		"modify and the full scope": `{"access_token": "ya29.access", "expires_in": 3599, "refresh_token": "1//grant", "scope": "https://www.googleapis.com/auth/gmail.modify https://mail.google.com/"}`,
		"no scope":                  `{"access_token": "ya29.access", "expires_in": 3599, "refresh_token": "1//grant"}`,
		"no refresh token":          `{"access_token": "ya29.access", "expires_in": 3599, "scope": "https://www.googleapis.com/auth/gmail.modify"}`,
		"no access token":           `{"expires_in": 3599, "refresh_token": "1//grant", "scope": "https://www.googleapis.com/auth/gmail.modify"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := grantFrom([]byte(body)); err == nil {
				t.Errorf("grantFrom accepted %s as %+v", name, got)
			}
		})
	}
}

// The outcome of a refresh, from the token endpoint's answers as Google documents them and the
// failures that get no answer. An error response, a 400 or a 401, is a refusal, and everything else
// that goes wrong failed (ADR-0097, RFC 6749 section 5.2).
func TestARefreshsOutcome(t *testing.T) {
	_, unusable := parseTokenResponse([]byte(`{"expires_in": 3599}`))
	answered := func(code int, status, body string) error {
		_, err := answer(code, status, []byte(body))
		return err
	}
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"success":                   {nil, "succeeded"},
		"revoked grant":             {answered(400, "400 Bad Request", `{"error": "invalid_grant", "error_description": "Token has been expired or revoked."}`), "refused"},
		"unknown client":            {answered(401, "401 Unauthorized", `{"error": "invalid_client", "error_description": "The OAuth client was not found."}`), "refused"},
		"refusal without its body":  {answered(400, "400 Bad Request", `<html></html>`), "refused"},
		"server error":              {answered(500, "500 Internal Server Error", `{"error": "internal_failure"}`), "failed"},
		"unavailable":               {answered(503, "503 Service Unavailable", ``), "failed"},
		"too many requests":         {answered(429, "429 Too Many Requests", `{"error": "rate_limit_exceeded"}`), "failed"},
		"forbidden":                 {answered(403, "403 Forbidden", ``), "failed"},
		"a success it cannot read":  {unusable, "failed"},
		"no answer":                 {errors.New("dial tcp 127.0.0.1:9: connect: connection refused"), "failed"},
		"a refusal wrapped further": {fmt.Errorf("refreshing: %w", answered(400, "400 Bad Request", `{"error": "invalid_grant"}`)), "refused"},
	} {
		if got := outcome(c.err); string(got) != c.want {
			t.Errorf("%s: the outcome is %q, want %q", name, got, c.want)
		}
	}
}

// A successful answer hands its body on, and any other names the status and Google's error, as the
// code exchange and the refresh both report it.
func TestTheTokenEndpointsAnswer(t *testing.T) {
	body, err := answer(200, "200 OK", []byte(`{"access_token": "access"}`))
	if err != nil || string(body) != `{"access_token": "access"}` {
		t.Errorf("a success returned %q, %v", body, err)
	}
	for _, c := range []struct {
		code         int
		status, body string
		want         string
	}{
		{400, "400 Bad Request", `{"error": "invalid_grant", "error_description": "Bad Request"}`, "gmail: the token endpoint answered 400 Bad Request: invalid_grant Bad Request"},
		{502, "502 Bad Gateway", `<html></html>`, "gmail: the token endpoint answered 502 Bad Gateway"},
	} {
		if _, err := answer(c.code, c.status, []byte(c.body)); err == nil || err.Error() != c.want {
			t.Errorf("answer %d returned %v, want %q", c.code, err, c.want)
		}
	}
}
