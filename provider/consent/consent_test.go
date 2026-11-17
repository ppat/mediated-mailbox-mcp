package consent_test

import (
	"net/http"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/provider/consent"
	"github.com/ppat/mediated-mailbox-mcp/provider/google"
)

// Gmail's consent is Google's, redirecting to the address it is given, and a provider whose accounts
// connect through no consent has none (ADR-0107).
func TestEachProvidersConsentByName(t *testing.T) {
	const redirect = "http://127.0.0.1:8085/"
	c, err := consent.For("gmail", http.DefaultClient, redirect)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := c.(google.Google); !ok || got != google.New(http.DefaultClient, redirect) {
		t.Errorf("the gmail consent is %#v, want Google's redirecting to %s", c, redirect)
	}
	if c, err := consent.For("jmap", http.DefaultClient, redirect); err == nil {
		t.Errorf("For(jmap) returned %#v, want no consent", c)
	}
}
