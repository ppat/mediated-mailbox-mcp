package gmail_test

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// consentQuery parses the consent page address for fixed inputs.
func consentQuery(t *testing.T) (*url.URL, url.Values) {
	t.Helper()
	u, err := url.Parse(gmail.AuthorizationURL("client-id", "http://127.0.0.1:8080/", "the-state", "verifier"))
	if err != nil {
		t.Fatalf("AuthorizationURL is not a URL: %v", err)
	}
	return u, u.Query()
}

// The grant requests the modify scope and nothing else, so the token cannot permanently delete
// mail (ADR-0011). include_granted_scopes would add every scope the account granted this client
// before, so it must be absent too.
func TestTheConsentRequestsOnlyTheModifyScope(t *testing.T) {
	_, q := consentQuery(t)
	if diff := cmp.Diff([]string{"https://www.googleapis.com/auth/gmail.modify"}, q["scope"], compare.Options); diff != "" {
		t.Errorf("requested scope (-want +got):\n%s", diff)
	}
	if got, ok := q["include_granted_scopes"]; ok {
		t.Errorf("the consent folds in previously granted scopes: include_granted_scopes=%q", got)
	}
}

// The consent asks Google for an offline grant, bound to the PKCE verifier, whose code returns to
// the given loopback address.
func TestTheConsentAsksForAnOfflineGrant(t *testing.T) {
	u, q := consentQuery(t)
	if got := u.Scheme + "://" + u.Host + u.Path; got != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Errorf("consent endpoint = %q", got)
	}
	want := url.Values{
		"access_type":           {"offline"},
		"client_id":             {"client-id"},
		"code_challenge":        {"iMnq5o6zALKXGivsnlom_0F5_WYda32GHkxlV7mq7hQ"},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
		"redirect_uri":          {"http://127.0.0.1:8080/"},
		"response_type":         {"code"},
		"scope":                 {"https://www.googleapis.com/auth/gmail.modify"},
		"state":                 {"the-state"},
	}
	if diff := cmp.Diff(want, q, compare.Options); diff != "" {
		t.Errorf("consent query (-want +got):\n%s", diff)
	}
}
