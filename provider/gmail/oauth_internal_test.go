package gmail

import (
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
