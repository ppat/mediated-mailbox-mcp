package fake_test

import (
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
)

// A list of adapters for a test serves each provider it is given, with the facts it is given, and
// makes each token from the credential it is handed, a token a test plays the provider's part on. The
// connections' tests build mailboxes over such tokens through a caller (ADR-0043).
func TestAdaptersServeTheProvidersTheyAreGiven(t *testing.T) {
	list := fake.Adapters(func(account string, _ *fake.Token) (*fake.Throttled, error) {
		f, err := fake.New(fake.Config{Account: account})
		return fake.Throttle(f, fake.Never(), time.Now), err
	}, fake.Provider{Name: "gmail", UsesClient: true}, fake.Provider{Name: "fastmail"})

	for name, usesClient := range map[string]bool{"gmail": true, "fastmail": false} {
		if facts, ok := list.Facts(name); !ok || facts.UsesClient != usesClient || facts.Ceiling != 100 {
			t.Errorf("the facts of %s are %+v, %v", name, facts, ok)
		}
	}
	if _, ok := list.Facts("jmap"); ok {
		t.Error("a provider the list was not given has facts")
	}
	client := mail.OAuthClient{ID: "client-id", Secret: "client-secret"}
	token, err := list.NewToken("gmail", client, []byte("credential"))
	if err != nil {
		t.Fatal(err)
	}
	if string(token.Credential()) != "credential" {
		t.Errorf("the token holds %q", token.Credential())
	}
	attempt := mail.AuthAttempt{At: 1, Outcome: mail.AuthRefused}
	made := &fake.Token{}
	made.Rotate([]byte("rotated"))
	made.Attempted(attempt)
	read := 0
	made.AtAttemptRead(func() { read++ })
	if string(made.Credential()) != "rotated" || made.LastAttempt() != attempt || read != 1 {
		t.Errorf("a token holds %q and %+v after a rotation and an attempt", made.Credential(), made.LastAttempt())
	}
	if _, err := list.NewToken("jmap", client, nil); err == nil {
		t.Error("a token was made for a provider the list does not serve")
	}
}
