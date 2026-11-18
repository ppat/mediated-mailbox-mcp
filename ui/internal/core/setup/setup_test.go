package setup_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
)

// none is no refusal's code.
const none refusal.Code = ""

// code is a refusal's code, none for no refusal.
func code(r *refusal.Refusal) refusal.Code {
	if r == nil {
		return none
	}
	return r.Code
}

// A client's name is shaped like a project ID and is never new, so it is one path segment its screens
// can be reached at (docs/UI.md sections 5 and 8.11, VERIFICATIONS, the client name row).
func TestAClientNameIsShapedLikeAProjectID(t *testing.T) {
	for _, name := range []string{"mediated-mailbox", "abcdef", "a23456789012345678901234567890", "home-2"} {
		if got := code(setup.ClientName(name)); got != none {
			t.Errorf("%q refused with %q", name, got)
		}
	}
	for _, name := range []string{
		"new", "", "abcde", "a234567890123456789012345678901", "1abcdef", "-abcdef",
		"Mediated-mailbox", "mediated.mailbox", "mediated/mailbox", "mediated mailbox", "médiated",
	} {
		if got := code(setup.ClientName(name)); got != refusal.NameRefused {
			t.Errorf("%q gave %q, want %q", name, got, refusal.NameRefused)
		}
	}
	project := "123456789012"
	if got := code(setup.ProjectID(&project)); got != refusal.ProjectRefused {
		t.Errorf("a project number gave %q", got)
	}
	if got := code(setup.ProjectID(nil)); got != none {
		t.Errorf("no project ID gave %q", got)
	}
}

// A lowered target is above 5% and at most 50% of the declared ceiling (ADR-0024).
func TestALoweredTargetStaysInItsRange(t *testing.T) {
	for _, f := range []float64{0.051, 0.3, 0.5} {
		if got := code(setup.Target(&f)); got != none {
			t.Errorf("%v refused with %q", f, got)
		}
	}
	for _, f := range []float64{0.05, 0, -0.1, 0.5001, 1} {
		if got := code(setup.Target(&f)); got != refusal.TargetRefused {
			t.Errorf("%v gave %q", f, got)
		}
	}
	if got := code(setup.Target(nil)); got != none {
		t.Errorf("clearing the target gave %q", got)
	}
}

// A mailbox address has a local part and a domain.
func TestAMailboxHasALocalPartAndADomain(t *testing.T) {
	for _, m := range []string{"", "jo", "@gmail.com", "jo@", " jo@gmail.com"} {
		if code(setup.Mailbox(m)) != refusal.MailboxRefused {
			t.Errorf("%q was accepted", m)
		}
	}
	if code(setup.Mailbox("jo@gmail.com")) != none {
		t.Error("an address was refused")
	}
}

// An attempt finishes nothing from the instant it expires, and anything before it (ADR-0111,
// VERIFICATIONS, the attempt rows).
func TestAnAttemptExpiresAtItsExpiry(t *testing.T) {
	const expires = int64(1000)
	for now, want := range map[int64]refusal.Code{0: none, 999: none, 1000: refusal.AttemptExpired, 5000: refusal.AttemptExpired} {
		if got := code(setup.Expired(expires, now)); got != want {
			t.Errorf("at %d: %q, want %q", now, got, want)
		}
	}
}

// A pasted redirect finishes only the attempt whose state it carries, with a code and no error
// (docs/UI.md section 8.12, VERIFICATIONS, the attempt rows).
func TestARedirectFinishesOnlyItsOwnAttempt(t *testing.T) {
	const state = "the-state"
	for name, c := range map[string]struct {
		r    mail.Redirect
		want refusal.Code
	}{
		"its own code":              {mail.Redirect{State: state, Code: "c"}, none},
		"another attempt's code":    {mail.Redirect{State: "other", Code: "c"}, refusal.WrongAttempt},
		"a code with no state":      {mail.Redirect{Code: "c"}, refusal.WrongAttempt},
		"another attempt's refusal": {mail.Redirect{State: "other", Error: "access_denied"}, refusal.WrongAttempt},
		"declined":                  {mail.Redirect{State: state, Error: "access_denied"}, refusal.ConsentDeclined},
		"no code":                   {mail.Redirect{State: state}, refusal.NoCode},
		"neither code nor state":    {mail.Redirect{}, refusal.NoCode},
	} {
		if got := code(setup.Redirect(c.r, state)); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}

// A client is refused, before the provider is asked, for an empty identifier or secret, a project ID
// not shaped like one, a name a stored client holds and an identifier a stored client of its provider
// holds, the first stored client either names deciding it. A replacement is refused for a name no
// client of the provider holds, and for an identifier another of its clients holds (docs/UI.md
// section 8.11, ADR-0106).
func TestAClientIsRefusedForWhatAStoredClientHolds(t *testing.T) {
	stored := []setup.Client{{Name: "household-mail", Provider: "gmail", ID: "h.apps"}, {Name: "spare-client", Provider: "gmail", ID: "s.apps"}}
	project := "12345"
	cases := []struct {
		name string
		got  *refusal.Refusal
		want *refusal.Refusal
	}{
		{"a new client", setup.NewClient("id", "secret", nil), nil},
		{"no identifier", setup.NewClient("", "secret", nil), refusal.Of(refusal.ClientRefused, "the client identifier and the client secret are both required")},
		{"no secret", setup.NewClient("id", "", nil), refusal.Of(refusal.ClientRefused, "the client identifier and the client secret are both required")},
		{"a project number", setup.NewClient("id", "secret", &project), setup.ProjectID(&project)},
		{"a free name and identifier", setup.Added(stored, setup.Client{Name: "other-client", Provider: "gmail", ID: "o.apps"}), nil},
		{"a held name", setup.Added(stored, setup.Client{Name: "spare-client", Provider: "gmail", ID: "o.apps"}), refusal.Of(refusal.NameTaken, "another client holds this name")},
		{"a held identifier", setup.Added(stored, setup.Client{Name: "other-client", Provider: "gmail", ID: "s.apps"}), refusal.Of(refusal.ClientExists, "this client is already set up as spare-client")},
		{"the first stored client decides", setup.Added(stored, setup.Client{Name: "spare-client", Provider: "gmail", ID: "h.apps"}), refusal.Of(refusal.ClientExists, "this client is already set up as household-mail")},
		{"another provider's identifier", setup.Added(stored, setup.Client{Name: "other-client", Provider: "jmap", ID: "h.apps"}), nil},
		{"a replacement", setup.Replaced(stored, setup.Client{Name: "household-mail", Provider: "gmail", ID: "n.apps"}), nil},
		{"a replacement keeping its identifier", setup.Replaced(stored, setup.Client{Name: "household-mail", Provider: "gmail", ID: "h.apps"}), nil},
		{"a replacement of an unknown client", setup.Replaced(stored, setup.Client{Name: "absent-client", Provider: "gmail", ID: "n.apps"}), setup.UnknownClient()},
		{"a replacement under another provider", setup.Replaced(stored, setup.Client{Name: "household-mail", Provider: "jmap", ID: "n.apps"}), setup.UnknownClient()},
		{"a replacement to another client's identifier", setup.Replaced(stored, setup.Client{Name: "household-mail", Provider: "gmail", ID: "s.apps"}), refusal.Of(refusal.ClientExists, "this client is already set up as spare-client")},
		{"the name key", setup.ClientConflict(setup.ClientNameKey), refusal.Of(refusal.NameTaken, "another client holds this name")},
		{"the identifier key", setup.ClientConflict(setup.ClientIDKey), refusal.Of(refusal.ClientExists, "this client is already set up under another name")},
		{"another key", setup.ClientConflict("another_key"), nil},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, c.got); diff != "" {
			t.Errorf("%s (-want +got):\n%s", c.name, diff)
		}
	}
}

// A re-authorization goes through the client the request names, or the account's own, of the
// account's provider. It is a move when the client is not the account's own. Its mailbox is the one
// the account remembers, and a request naming one while the account remembers its own, or naming
// none while it remembers none, is refused (docs/UI.md section 8.12, ADR-0080, ADR-0106,
// VERIFICATIONS, the move row).
func TestAReauthorizationIsDecidedFromTheAccountItStartsFor(t *testing.T) {
	remembered, other, bad := "jo@gmail.com", "kim@gmail.com", "kim"
	own := setup.Client{Name: "household-mail", Provider: "gmail", ID: "h.apps"}
	spare := setup.Client{Name: "spare-client", Provider: "gmail", ID: "s.apps"}
	held := setup.Held{Provider: "gmail", Client: "household-mail", Mailbox: &remembered}
	forgotten := setup.Held{Provider: "gmail", Client: "household-mail"}
	type decision struct {
		Kind, Mailbox string
		Refusal       *refusal.Refusal
	}
	decide := func(h setup.Held, c setup.Client, mailbox *string) decision {
		kind, address, r := setup.Reauthorization(h, c, mailbox)
		return decision{kind, address, r}
	}
	cases := []struct {
		name      string
		got, want decision
	}{
		{"its own client", decide(held, own, nil), decision{setup.Reauthorize, remembered, nil}},
		{"another client", decide(held, spare, nil), decision{setup.Move, remembered, nil}},
		{"another provider's client", decide(held, setup.Client{Name: "x-client", Provider: "jmap"}, nil), decision{Refusal: refusal.Of(refusal.ClientWrongProvider, "the client belongs to another provider than the account's")}},
		{"a mailbox named while one is remembered", decide(held, own, &other), decision{Refusal: refusal.Of(refusal.MailboxRemembered, "the account remembers its mailbox, so the request names none")}},
		{"no mailbox named while none is remembered", decide(forgotten, own, nil), decision{Refusal: refusal.Of(refusal.MailboxRequired, "the account remembers no mailbox, so the request names it")}},
		{"a mailbox named while none is remembered", decide(forgotten, own, &other), decision{setup.Reauthorize, other, nil}},
		{"a mailbox not shaped like one", decide(forgotten, own, &bad), decision{Refusal: setup.Mailbox(bad)}},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, c.got); diff != "" {
			t.Errorf("%s (-want +got):\n%s", c.name, diff)
		}
	}
	requested := "spare-client"
	for name, c := range map[string]struct {
		held      setup.Held
		requested *string
		want      string
		refused   bool
	}{
		"its own":       {held, nil, "household-mail", false},
		"the requested": {held, &requested, "spare-client", false},
		"none":          {setup.Held{Provider: "gmail"}, nil, "", true},
	} {
		got, r := setup.ClientOf(c.held, c.requested)
		if got != c.want || (r != nil) != c.refused || r != nil && r.Code != refusal.UnknownClient {
			t.Errorf("%s: %q, %v", name, got, r)
		}
	}
	stored := []setup.Client{own, {Name: "x-client", Provider: "jmap", ID: "x"}}
	if got, r := setup.Named(stored, "household-mail", []string{"gmail"}); got != own || r != nil {
		t.Errorf("a held name of a known provider: %v, %v", got, r)
	}
	for name, n := range map[string]string{"a name no client holds": "absent", "a provider with no consent": "x-client"} {
		if _, r := setup.Named(stored, n, []string{"gmail"}); r == nil || r.Code != refusal.UnknownClient || r.Message != "no client holds this name" {
			t.Errorf("%s: %v", name, r)
		}
	}
}

// A finish is refused before its code is exchanged when an account took a connection's identifier
// or the attempt's client changed since it started, and its transaction's failure is the client's
// when the client changed, the accounts key refused the identifier or the database's check refused
// it, and the database's otherwise (docs/UI.md section 8.12, ADR-0091, ADR-0106, VERIFICATIONS, the
// attempt rows).
func TestAFinishIsRefusedForWhatChangedSinceItsAttemptStarted(t *testing.T) {
	client := setup.Client{Name: "household-mail", Provider: "gmail", ID: "h.apps"}
	replaced := setup.Client{Name: "household-mail", Provider: "gmail", ID: "n.apps"}
	changed := refusal.Of(refusal.ClientChanged, "the client household-mail changed while this was in progress")
	cases := []struct {
		name string
		got  *refusal.Refusal
		want *refusal.Refusal
	}{
		{"a free connection", setup.StillFree(setup.Connect, "jo", []string{"kim"}, []setup.Client{client}, client), nil},
		{"a taken identifier", setup.StillFree(setup.Connect, "jo", []string{"jo"}, []setup.Client{client}, client), setup.IdentifierTaken()},
		{"a re-authorization of a listed account", setup.StillFree(setup.Reauthorize, "jo", []string{"jo"}, []setup.Client{client}, client), nil},
		{"a replaced client", setup.StillFree(setup.Move, "jo", nil, []setup.Client{replaced}, client), changed},
		{"a removed client", setup.StillFree(setup.Connect, "jo", nil, nil, client), changed},
		{"the client changed", setup.Stored(true, setup.Violation{}), refusal.Of(refusal.ClientChanged, "the attempt's client changed while this was in progress")},
		{"the accounts key", setup.Stored(false, setup.Violation{Constraint: setup.AccountKey, Unique: true}), setup.IdentifierTaken()},
		{"another key", setup.Stored(false, setup.Violation{Constraint: "other_key", Unique: true}), nil},
		{"the identifier's check", setup.Stored(false, setup.Violation{Constraint: "accounts_account_id_addressable", Check: true}), setup.IdentifierRefusal()},
		{"the database's failure", setup.Stored(false, setup.Violation{}), nil},
		{"a free identifier", setup.Unclaimed([]string{"kim"}, "jo"), nil},
		{"a listed identifier", setup.Unclaimed([]string{"jo"}, "jo"), refusal.Of(refusal.IdentifierTaken, "an account already holds this identifier")},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, c.got); diff != "" {
			t.Errorf("%s (-want +got):\n%s", c.name, diff)
		}
	}
	for name, c := range map[string]struct {
		identity []setup.Client
		want     bool
	}{
		"unchanged": {[]setup.Client{client}, false},
		"replaced":  {[]setup.Client{replaced}, true},
		"removed":   {nil, true},
	} {
		if got := setup.ClientChanged(c.identity, client); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}
