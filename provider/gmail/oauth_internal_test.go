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
			tokenResponse{AccessToken: "ya29.access", ExpiresIn: 3599 * time.Second, RefreshToken: "1//rotated"},
		},
		{
			"refresh that does not rotate",
			`{"access_token": "ya29.access", "expires_in": 3599, "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`,
			tokenResponse{AccessToken: "ya29.access", ExpiresIn: 3599 * time.Second},
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
