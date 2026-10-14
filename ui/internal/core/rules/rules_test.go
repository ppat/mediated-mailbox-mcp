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

// A write is refused for what the snapshot's validation refuses, and for an identifier no path segment
// can hold, while a word the policy screens' addresses use is an identifier like any other, since a
// rule's identifier follows rules or base in its address (docs/UI.md sections 5, 8.7 and 8.14).
func TestCheck(t *testing.T) {
	cases := []struct {
		name  string
		rules []rules.Rule
		want  []rules.Problem
	}{
		{"a valid rule", []rules.Rule{rule("operator.schwab.com", "schwab.com", "schwabmail.com")}, nil},
		{"an empty identifier", []rules.Rule{rule("", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: rules.BlankIdentifier}}},
		{"an identifier with surrounding space", []rules.Rule{rule(" x", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: rules.BlankIdentifier}}},
		{"an identifier the scope holds", []rules.Rule{rule("x", "a.example"), rule("x", "b.example")}, []rules.Problem{{Rule: 1, Kind: rules.RepeatedIdentifier}}},
		{"no suffix", []rules.Rule{rule("x")}, []rules.Problem{{Rule: 0, Kind: rules.NoSuffix}}},
		{"a suffix not shaped like a domain name", []rules.Rule{rule("x", "a.example", "*.b.example")}, []rules.Problem{{Rule: 0, Kind: rules.InvalidSuffix, Suffix: "*.b.example"}}},
		{"a word an account's policy screens use", []rules.Rule{rule("pick", "a.example"), rule("base", "b.example"), rule("new", "c.example"), rule("rules", "d.example")}, nil},
		{"a word the base policy's screens use", []rules.Rule{rule("history", "a.example"), rule("import", "b.example"), rule("rules", "c.example")}, nil},
		{"an identifier holding a hyphen", []rules.Rule{rule("operator-bank", "bank.example")}, nil},
		{"a dot", []rules.Rule{rule(".", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"two dots", []rules.Rule{rule("..", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"a slash", []rules.Rule{rule("a/b", "a.example")}, []rules.Problem{{Rule: 0, Kind: rules.ReservedIdentifier}}},
		{"several problems, in rule order", []rules.Rule{rule("..", "bad suffix"), rule("")}, []rules.Problem{
			{Rule: 0, Kind: rules.InvalidSuffix, Suffix: "bad suffix"},
			{Rule: 0, Kind: rules.ReservedIdentifier},
			{Rule: 1, Kind: rules.BlankIdentifier},
			{Rule: 1, Kind: rules.NoSuffix},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, rules.Check(c.rules), compare.Options); diff != "" {
				t.Errorf("Check (-want +got):\n%s", diff)
			}
		})
	}
}

// A file stating another class than restricted is refused, as the snapshot's validation refuses it.
func TestCheckClasses(t *testing.T) {
	got := rules.CheckClasses([]rules.Rule{rule("x", "a.example")}, []string{"normal"})
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

var senders = []rules.Stored{
	{Domain: "schwab.com", Messages: 10},
	{Domain: "mail.schwab.com", Messages: 4},
	{Domain: "notschwab.com", Messages: 7},
	{Domain: "xn--bcher-kva.example", Messages: 2},
	{Domain: "com", Messages: 1},
}

// A suffix matches a sender at its own domain or a subdomain, at a label boundary, in the form the
// classifier compares, a sender counting once however many suffixes match it. A domain the classifier
// cannot classify matches nothing (ADR-0004).
func TestMatch(t *testing.T) {
	s := rules.NewSenders(senders, nil, lookups)
	if diff := cmp.Diff(rules.Count{Senders: 2, Messages: 14}, s.Match([]string{"schwab.com", "mail.schwab.com"}), compare.Options); diff != "" {
		t.Errorf("Match of schwab.com (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{Senders: 1, Messages: 2}, s.Match([]string{"Bücher.example"}), compare.Options); diff != "" {
		t.Errorf("Match of a suffix in Unicode (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(rules.Count{Senders: 3, Messages: 21}, s.Match([]string{"com"}), compare.Options); diff != "" {
		t.Errorf("Match of a public suffix, every sender under it but the one with no registrable domain (-want +got):\n%s", diff)
	}
	if got := s.Matched([]string{"schwab.com"}); len(got) != 2 || got[0].Domain != "schwab.com" || got[1].Domain != "mail.schwab.com" {
		t.Errorf("Matched = %+v, want schwab.com and mail.schwab.com", got)
	}
}

// What a change releases is the senders the policy before restricts and the policy after does not, so
// a suffix another rule still covers releases nothing (docs/UI.md section 8.7).
func TestReleased(t *testing.T) {
	s := rules.NewSenders(senders, nil, lookups)
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

// The rule restricting a sender is the first the classifier meets, a sender no rule matches is normal
// with none, and a domain the classifier cannot read is restricted with none, as Classify decides
// each (ADR-0004, ADR-0136).
func TestClassOf(t *testing.T) {
	policy := []rules.Rule{rule("base.schwab", "schwab.com"), rule("own.mail", "mail.schwab.com")}
	type class struct {
		Restricted bool
		Rule       string
	}
	for domain, want := range map[string]class{
		"mail.schwab.com": {true, "base.schwab"},
		"notschwab.com":   {false, ""},
		"com":             {true, ""},
		"":                {true, ""},
	} {
		restricted, rule := rules.ClassOf(domain, rules.Names(policy, lookups), lookups)
		if got := (class{restricted, rule}); got != want {
			t.Errorf("ClassOf(%q) = %+v, want %+v", domain, got, want)
		}
	}
}

// Each sender's class is the composed policy's as the request read it, never a class stored with it,
// so the senders restricted, their messages and the normal domains follow the policy given (ADR-0136).
// A domain the classifier cannot read is restricted.
func TestEachSenderIsClassifiedUnderThePolicyRead(t *testing.T) {
	cases := []struct {
		name       string
		policy     []rules.Rule
		restricted rules.Count
		normal     []string
	}{
		{"no rule", nil, rules.Count{Senders: 1, Messages: 1}, []string{"schwab.com", "mail.schwab.com", "notschwab.com", "xn--bcher-kva.example"}},
		{"a rule for schwab.com", []rules.Rule{rule("base.schwab", "schwab.com")}, rules.Count{Senders: 3, Messages: 15}, []string{"notschwab.com", "xn--bcher-kva.example"}},
		{"a rule for a suffix in Unicode", []rules.Rule{rule("books", "Bücher.example")}, rules.Count{Senders: 2, Messages: 3}, []string{"schwab.com", "mail.schwab.com", "notschwab.com"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := rules.NewSenders(senders, rules.Names(c.policy, lookups), lookups)
			if diff := cmp.Diff(c.restricted, s.Restricted(), compare.Options); diff != "" {
				t.Errorf("Restricted (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.normal, s.Normal(), compare.Options); diff != "" {
				t.Errorf("Normal (-want +got):\n%s", diff)
			}
		})
	}
	if got := rules.NewSenders(nil, nil, lookups).Normal(); got == nil {
		t.Error("Normal over no sender is nil, which a statement reads as no filter")
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
