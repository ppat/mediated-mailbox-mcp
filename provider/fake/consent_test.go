package fake_test

import (
	"errors"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
)

// The fake's consent exchanges a code once, for the client it was issued to with that client's secret
// and the verifier of its consent page, as Google's token endpoint does, and reads the mailbox that
// granted it.
func TestTheFakeConsentExchangesACodeOnce(t *testing.T) {
	c := fake.NewConsent("http://127.0.0.1:47823/")
	c.AddClient("id", "secret")
	client := mail.OAuthClient{ID: "id", Secret: "secret"}
	if err := c.CheckClient(t.Context(), client); err != nil {
		t.Fatalf("the client was refused: %v", err)
	}
	if err := c.CheckClient(t.Context(), mail.OAuthClient{ID: "id", Secret: "other"}); !errors.Is(err, mail.ErrClientRefused) {
		t.Errorf("a wrong secret: %v, want %v", err, mail.ErrClientRefused)
	}
	grant := func(verifier string) mail.Redirect {
		t.Helper()
		address, err := c.Grant(c.ConsentAddress("id", "state-"+verifier, verifier, "jo@gmail.com"), "jo@gmail.com", consent.Scope)
		if err != nil {
			t.Fatal(err)
		}
		r, err := c.ReadRedirect(address)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := grant("verifier-one-0000000000000000000000000000000000")
	got, err := c.Finish(t.Context(), client, r.Code, "verifier-one-0000000000000000000000000000000000")
	if err != nil || got.Mailbox != "jo@gmail.com" || got.Credential == "" {
		t.Fatalf("the exchange gave %+v, %v", got, err)
	}
	if _, err := c.Finish(t.Context(), client, r.Code, "verifier-one-0000000000000000000000000000000000"); !errors.Is(err, mail.ErrCodeRefused) {
		t.Errorf("a used code: %v, want %v", err, mail.ErrCodeRefused)
	}
	r = grant("verifier-two-0000000000000000000000000000000000")
	if _, err := c.Finish(t.Context(), client, r.Code, "another-verifier-000000000000000000000000000000"); !errors.Is(err, mail.ErrCodeRefused) {
		t.Errorf("another verifier: %v, want %v", err, mail.ErrCodeRefused)
	}
	c.DisableAPI("id")
	r = grant("verifier-three-00000000000000000000000000000000")
	if _, err := c.Finish(t.Context(), client, r.Code, "verifier-three-00000000000000000000000000000000"); !errors.Is(err, mail.ErrAPIDisabled) {
		t.Errorf("a disabled API: %v, want %v", err, mail.ErrAPIDisabled)
	}
}
