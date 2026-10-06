package rules_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
)

var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

func rule(id string, suffixes ...string) rules.Rule { return rules.Rule{ID: id, Suffixes: suffixes} }

// A write is refused for what the snapshot's validation refuses, and for an identifier its scope's
// screens could not reach. Each scope has its own route words, and an identifier repeated only across
// two writes of different scopes is never seen together here (docs/UI.md sections 8.7 and 8.14).
func TestCheck(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		rules []rules.Rule
		want  []rules.Problem
	}{
		{"a valid rule", rules.Account, []rules.Rule{rule("operator.schwab.com", "schwab.com", "schwabmail.com")}, nil},
		{"an empty identifier", rules.Account, []rules.Rule{rule("", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: rules.BlankIdentifier}}},
		{"an identifier with surrounding space", rules.Base, []rules.Rule{rule(" x", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: rules.BlankIdentifier}}},
		{"an identifier the scope holds", rules.Account, []rules.Rule{rule("x", "a.example"), rule("x", "b.example")}, []rules.Problem{{Rule: 1, Kind: rules.RepeatedIdentifier}}},
		{"no suffix", rules.Account, []rules.Rule{rule("x")}, []rules.Problem{{Rule: 0, Kind: rules.NoSuffix}}},
		{"a suffix not shaped like a domain name", rules.Account, []rules.Rule{rule("x", "a.example", "*.b.example")}, []rules.Problem{{Rule: 0, Kind: rules.InvalidSuffix, Suffix: "*.b.example"}}},
		{"an account's route word", rules.Account, []rules.Rule{rule("pick", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"base is an account's route word", rules.Account, []rules.Rule{rule("base", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"base is no route word of the base policy", rules.Base, []rules.Rule{rule("base", "a.example")}, nil},
		{"pick is no route word of the base policy", rules.Base, []rules.Rule{rule("pick", "a.example")}, nil},
		{"the base policy's route word", rules.Base, []rules.Rule{rule("history", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"a dot", rules.Base, []rules.Rule{rule(".", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"two dots", rules.Account, []rules.Rule{rule("..", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"a slash", rules.Account, []rules.Rule{rule("a/b", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"several problems, in rule order", rules.Account, []rules.Rule{rule("new", "bad suffix"), rule("")}, []rules.Problem{
			{Rule: 0, Kind: rules.InvalidSuffix, Suffix: "bad suffix"},
			{Rule: 0, Kind: rules.ReservedIdentifier},
			{Rule: 1, Kind: rules.BlankIdentifier},
			{Rule: 1, Kind: rules.NoSuffix},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, rules.Check(c.scope, c.rules), compare.Options); diff != "" {
				t.Errorf("Check (-want +got):\n%s", diff)
			}
		})
	}
}

// A file stating another class than restricted is refused, as the snapshot's validation refuses it.
func TestCheckClasses(t *testing.T) {
	got := rules.CheckClasses(rules.Account, []rules.Rule{rule("x", "a.example")}, []string{"normal"})
	if diff := cmp.Diff([]rules.Problem{{Rule: 0, Kind: rules.OtherClass}}, got, compare.Options); diff != "" {
		t.Errorf("CheckClasses (-want +got):\n%s", diff)
	}
}

// An import makes the stored rules equal to the file. A rule in the file alone is added, one stored
// alone is lifted, one in both whose suffixes differ as sets is edited, and each lifted rule and each
// removed suffix counts one lift (ADR-0110).
func TestCompare(t *testing.T) {
	stored := []rules.Rule{
		rule("keep", "keep.example", "also.example"),
		rule("grow", "grow.example"),
		rule("shrink", "shrink.example", "gone.example"),
		rule("drop", "drop.example"),
	}
	file := []rules.Rule{
		rule("new", "new.example"),
		rule("shrink", "shrink.example", "swap.example"),
		rule("keep", "also.example", "keep.example"),
		rule("grow", "grow.example", "grown.example"),
	}
	got := rules.Compare(stored, file)
	want := rules.Diff{
		Added: []rules.Rule{rule("new", "new.example")},
		Edited: []rules.Edit{
			{ID: "grow", Before: []string{"grow.example"}, After: []string{"grow.example", "grown.example"}, Added: []string{"grown.example"}},
			{ID: "shrink", Before: []string{"shrink.example", "gone.example"}, After: []string{"shrink.example", "swap.example"}, Added: []string{"swap.example"}, Removed: []string{"gone.example"}},
		},
		Lifted:    []rules.Rule{rule("drop", "drop.example")},
		Unchanged: 1,
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Compare (-want +got):\n%s", diff)
	}
	if got.Lifts() != 2 || got.Changes() != 4 {
		t.Errorf("the diff lifts %d and changes %d, want 2 and 4", got.Lifts(), got.Changes())
	}
	if c := rules.Confirmation(got.Lifts()); c != "lift 2" {
		t.Errorf("Confirmation = %q, want lift 2", c)
	}
	if d := rules.Compare(stored, stored); d.Changes() != 0 || d.Unchanged != len(stored) {
		t.Errorf("a file equal to the stored rules changes %d and keeps %d, want none and every rule", d.Changes(), d.Unchanged)
	}
}

var senders = []rules.Sender{
	{Domain: "schwab.com", Restricted: true, Messages: 10},
	{Domain: "mail.schwab.com", Restricted: false, Messages: 4},
	{Domain: "notschwab.com", Restricted: false, Messages: 7},
	{Domain: "xn--bcher-kva.example", Restricted: false, Messages: 2},
	{Domain: "com", Restricted: true, Messages: 1},
}

// A suffix matches a sender at its own domain or a subdomain, at a label boundary, in the form the
// classifier compares, a sender counting once however many suffixes match it. A domain the classifier
// cannot classify matches nothing (ADR-0004).
func TestMatch(t *testing.T) {
	s := rules.NewSenders(senders, lookups)
	if diff := cmp.Diff(rules.Count{Senders: 2, Messages: 14, Restricted: 1}, s.Match([]string{"schwab.com", "mail.schwab.com"}), compare.Options); diff != "" {
		t.Errorf("Match of schwab.com (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{Senders: 1, Messages: 2}, s.Match([]string{"Bücher.example"}), compare.Options); diff != "" {
		t.Errorf("Match of a suffix in Unicode (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{Senders: 3, Messages: 21, Restricted: 1}, s.Match([]string{"com"}), compare.Options); diff != "" {
		t.Errorf("Match of a public suffix, every sender under it but the one with no registrable domain (-want +got):\n%s", diff)
	}
	if got := s.Matched([]string{"schwab.com"}); len(got) != 2 || got[0].Domain != "schwab.com" || got[1].Domain != "mail.schwab.com" {
		t.Errorf("Matched = %+v, want schwab.com and mail.schwab.com", got)
	}
}

// What a change releases is the senders the policy before restricts and the policy after does not, so
// a suffix another rule still covers releases nothing (docs/UI.md section 8.7).
func TestReleased(t *testing.T) {
	s := rules.NewSenders(senders, lookups)
	before := []string{"schwab.com", "notschwab.com", "mail.schwab.com"}
	if diff := cmp.Diff(rules.Count{Senders: 1, Messages: 7}, s.Released(before, []string{"schwab.com", "mail.schwab.com"}), compare.Options); diff != "" {
		t.Errorf("lifting notschwab.com (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{}, s.Released(before, []string{"schwab.com", "notschwab.com"}), compare.Options); diff != "" {
		t.Errorf("lifting a suffix a parent suffix still covers (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{Senders: 3, Messages: 21}, s.Released(before, nil), compare.Options); diff != "" {
		t.Errorf("lifting everything (-want +got):\n%s", diff)
	}
}

// The rule restricting a sender is the first the classifier meets, and a sender no rule matches has
// none.
func TestRestrictedBy(t *testing.T) {
	policy := []rules.Rule{rule("base.schwab", "schwab.com"), rule("own.mail", "mail.schwab.com")}
	for domain, want := range map[string]string{"mail.schwab.com": "base.schwab", "notschwab.com": "", "": ""} {
		if got := rules.RestrictedBy(domain, policy, lookups); got != want {
			t.Errorf("RestrictedBy(%q) = %q, want %q", domain, got, want)
		}
	}
}

// A suffix with no registrable domain under the public-suffix list is a public suffix.
func TestPublicSuffix(t *testing.T) {
	for suffix, want := range map[string]bool{"com": true, "co.uk": true, "schwab.com": false, "mail.schwab.com": false} {
		if got := rules.PublicSuffix(suffix, lookups); got != want {
			t.Errorf("PublicSuffix(%q) = %v, want %v", suffix, got, want)
		}
	}
}
