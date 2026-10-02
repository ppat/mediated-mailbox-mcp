package setup_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
)

// A client's name is shaped like a project ID and is never new, so it is one path segment its screens
// can be reached at (docs/UI.md sections 5 and 8.11, VERIFICATIONS, the client name row).
func TestAClientNameIsShapedLikeAProjectID(t *testing.T) {
	for _, name := range []string{"mediated-mailbox", "abcdef", "a23456789012345678901234567890", "home-2"} {
		if got := setup.ClientName(name); got != setup.None {
			t.Errorf("%q refused with %q", name, got)
		}
	}
	for _, name := range []string{
		"new", "", "abcde", "a234567890123456789012345678901", "1abcdef", "-abcdef",
		"Mediated-mailbox", "mediated.mailbox", "mediated/mailbox", "mediated mailbox", "médiated",
	} {
		if got := setup.ClientName(name); got != setup.NameRefused {
			t.Errorf("%q gave %q, want %q", name, got, setup.NameRefused)
		}
	}
	project := "123456789012"
	if got := setup.ProjectID(&project); got != setup.ProjectRefused {
		t.Errorf("a project number gave %q", got)
	}
	if got := setup.ProjectID(nil); got != setup.None {
		t.Errorf("no project ID gave %q", got)
	}
}

// An account identifier is refused when the mediator could not address it, or when a top-level path
// of the UI uses it (ADR-0087, docs/UI.md section 8.12, VERIFICATIONS, the two identifier rows).
func TestAnIdentifierEveryScreenCanReach(t *testing.T) {
	reserved := []string{"setup", "api", "main.js", "fonts"}
	for _, id := range []string{"", ".", "..", "/", "setup", "api", "main.js", "fonts"} {
		if got := setup.Identifier(id, reserved); got != setup.IdentifierRefused {
			t.Errorf("%q gave %q, want %q", id, got, setup.IdentifierRefused)
		}
	}
	for _, id := range []string{"personal", "...", "a.b", "setups", "main"} {
		if got := setup.Identifier(id, reserved); got != setup.None {
			t.Errorf("%q refused with %q", id, got)
		}
	}
}

// A lowered target is above 5% and at most 50% of the declared ceiling (ADR-0024).
func TestALoweredTargetStaysInItsRange(t *testing.T) {
	for _, f := range []float64{0.051, 0.3, 0.5} {
		if got := setup.Target(&f); got != setup.None {
			t.Errorf("%v refused with %q", f, got)
		}
	}
	for _, f := range []float64{0.05, 0, -0.1, 0.5001, 1} {
		if got := setup.Target(&f); got != setup.TargetRefused {
			t.Errorf("%v gave %q", f, got)
		}
	}
	if got := setup.Target(nil); got != setup.None {
		t.Errorf("clearing the target gave %q", got)
	}
}

// A mailbox address has a local part and a domain.
func TestAMailboxHasALocalPartAndADomain(t *testing.T) {
	for _, m := range []string{"", "jo", "@gmail.com", "jo@", " jo@gmail.com"} {
		if setup.Mailbox(m) != setup.MailboxRefused {
			t.Errorf("%q was accepted", m)
		}
	}
	if setup.Mailbox("jo@gmail.com") != setup.None {
		t.Error("an address was refused")
	}
}

// An attempt finishes nothing from the instant it expires, and anything before it (ADR-0111,
// VERIFICATIONS, the attempt rows).
func TestAnAttemptExpiresAtItsExpiry(t *testing.T) {
	const expires = int64(1000)
	for now, want := range map[int64]setup.Refusal{0: setup.None, 999: setup.None, 1000: setup.AttemptExpired, 5000: setup.AttemptExpired} {
		if got := setup.Expired(expires, now); got != want {
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
		want setup.Refusal
	}{
		"its own code":              {mail.Redirect{State: state, Code: "c"}, setup.None},
		"another attempt's code":    {mail.Redirect{State: "other", Code: "c"}, setup.WrongAttempt},
		"a code with no state":      {mail.Redirect{Code: "c"}, setup.WrongAttempt},
		"another attempt's refusal": {mail.Redirect{State: "other", Error: "access_denied"}, setup.WrongAttempt},
		"declined":                  {mail.Redirect{State: state, Error: "access_denied"}, setup.ConsentDeclined},
		"no code":                   {mail.Redirect{State: state}, setup.NoCode},
		"neither code nor state":    {mail.Redirect{}, setup.NoCode},
	} {
		if got := setup.Redirect(c.r, state); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}
