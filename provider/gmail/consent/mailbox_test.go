package consent_test

import (
	"errors"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
)

// The mailbox a grant belongs to is compared with the one named as Google identifies a mailbox,
// ignoring case, and for gmail.com and googlemail.com ignoring dots and taking the two domains as one
// (docs/UI.md section 8.12). Another mailbox, a dot outside Gmail's domains and an address sharing a
// prefix are refused.
func TestSameMailbox(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"jo.smith@gmail.com", "jo.smith@gmail.com"},
		{"Jo.Smith@Gmail.COM", "jo.smith@gmail.com"},
		{"josmith@gmail.com", "j.o.smith@gmail.com"},
		{"jo.smith@googlemail.com", "josmith@gmail.com"},
		{"Jo@Example.com", "jo@example.COM"},
	} {
		if !consent.SameMailbox(c.a, c.b) || !consent.SameMailbox(c.b, c.a) {
			t.Errorf("%s and %s were told apart, and name one mailbox", c.a, c.b)
		}
	}
	for _, c := range []struct{ a, b string }{
		{"jo.smith@gmail.com", "jo.smyth@gmail.com"},
		{"jo.smith@example.com", "josmith@example.com"},
		{"jo@gmail.com", "jo@gmail.com.evil"},
		{"jo@gmail.com", "jo@example.com"},
		{"", ""},
		{"@gmail.com", "@gmail.com"},
		{"jo@", "jo@"},
	} {
		if consent.SameMailbox(c.a, c.b) {
			t.Errorf("%q and %q were taken as one mailbox", c.a, c.b)
		}
	}
}

// The pasted address is the redirect address's, whatever its query, or it is refused. Its state,
// code and error are read as they are.
func TestParseRedirect(t *testing.T) {
	const redirect = "http://127.0.0.1:47823/"
	for address, want := range map[string]mail.Redirect{
		"http://127.0.0.1:47823/?state=s&code=c&scope=x":     {State: "s", Code: "c"},
		"  http://127.0.0.1:47823/?state=s&code=c  ":         {State: "s", Code: "c"},
		"127.0.0.1:47823/?state=s&code=c":                    {State: "s", Code: "c"},
		"http://127.0.0.1:47823?state=s&error=access_denied": {State: "s", Error: "access_denied"},
		"http://127.0.0.1:47823/?state=s":                    {State: "s"},
		"http://127.0.0.1:47823/?code=c%2Fd&state=s%20t":     {State: "s t", Code: "c/d"},
	} {
		got, err := consent.ParseRedirect(address, redirect)
		if err != nil || got != want {
			t.Errorf("%q read as %+v, %v, want %+v", address, got, err, want)
		}
	}
	for _, address := range []string{
		"https://127.0.0.1:47823/?state=s&code=c",
		"http://127.0.0.1:47824/?state=s&code=c",
		"http://localhost:47823/?state=s&code=c",
		"http://127.0.0.1:47823/other?state=s&code=c",
		"http://user@127.0.0.1:47823/?state=s&code=c",
		"https://accounts.google.com/o/oauth2/v2/auth?state=s&code=c",
		"",
		"%zz",
	} {
		if got, err := consent.ParseRedirect(address, redirect); !errors.Is(err, mail.ErrWrongAddress) {
			t.Errorf("%q read as %+v, %v, want %v", address, got, err, mail.ErrWrongAddress)
		}
	}
}

// The UI's consent redirects to the address it is given, and asks Google for the mailbox named first,
// and reads a pasted address against that same address.
func TestTheUIsConsentRedirectsToItsGivenAddress(t *testing.T) {
	g := consent.New(nil, "http://[::1]:5000/")
	got := g.ConsentAddress("id", "state", "verifier", "jo@gmail.com")
	if want := consent.AuthorizationURL("id", "http://[::1]:5000/", "state", "verifier", "jo@gmail.com"); got != want {
		t.Errorf("the consent address is %q, want %q", got, want)
	}
	if r, err := g.ReadRedirect("http://[::1]:5000/?state=s&code=c"); err != nil || r.Code != "c" {
		t.Errorf("the given address read as %+v, %v", r, err)
	}
	if _, err := g.ReadRedirect("http://127.0.0.1:47823/?state=s&code=c"); !errors.Is(err, mail.ErrWrongAddress) {
		t.Errorf("another loopback address: %v, want %v", err, mail.ErrWrongAddress)
	}
}
