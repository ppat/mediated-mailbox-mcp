package rules_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
)

var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

func rule(id string, suffixes ...string) rules.Rule { return rules.Rule{ID: id, Suffixes: suffixes} }

// prepared is the rules, in order, loaded as one account's composed policy and prepared with the real
// lookups.
func prepared(rs ...rules.Rule) classify.Policy {
	rows := make([]policy.Row, len(rs))
	for i, r := range rs {
		rows[i] = policy.Row{Account: "a", ID: r.ID, Class: policy.Restricted, DomainSuffixes: r.Suffixes}
	}
	s, err := policy.Load(rows)
	if err != nil {
		panic(err)
	}
	return classify.Prepare(s.For("a"), lookups)
}

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
		{"an empty identifier", []rules.Rule{rule("", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: refusal.BlankIdentifier}}},
		{"an identifier with surrounding space", []rules.Rule{rule(" x", "schwab.com")}, []rules.Problem{{Rule: 0, Kind: refusal.BlankIdentifier}}},
		{"an identifier the scope holds", []rules.Rule{rule("x", "a.example"), rule("x", "b.example")}, []rules.Problem{{Rule: 1, Kind: refusal.RepeatedIdentifier}}},
		{"no suffix", []rules.Rule{rule("x")}, []rules.Problem{{Rule: 0, Kind: refusal.NoSuffix}}},
		{"a suffix not shaped like a domain name", []rules.Rule{rule("x", "a.example", "*.b.example")}, []rules.Problem{{Rule: 0, Kind: refusal.InvalidSuffix, Suffix: "*.b.example"}}},
		{"a word an account's policy screens use", []rules.Rule{rule("pick", "a.example"), rule("base", "b.example"), rule("new", "c.example"), rule("rules", "d.example")}, nil},
		{"a word the base policy's screens use", []rules.Rule{rule("history", "a.example"), rule("import", "b.example"), rule("rules", "c.example")}, nil},
		{"an identifier holding a hyphen", []rules.Rule{rule("operator-bank", "bank.example")}, nil},
		{"a dot", []rules.Rule{rule(".", "a.example")}, []rules.Problem{{Rule: 0, Kind: refusal.ReservedIdentifier}}},
		{"two dots", []rules.Rule{rule("..", "a.example")}, []rules.Problem{{Rule: 0, Kind: refusal.ReservedIdentifier}}},
		{"a slash", []rules.Rule{rule("a/b", "a.example")}, []rules.Problem{{Rule: 0, Kind: refusal.ReservedIdentifier}}},
		{"several problems, in rule order", []rules.Rule{rule("..", "bad suffix"), rule("")}, []rules.Problem{
			{Rule: 0, Kind: refusal.InvalidSuffix, Suffix: "bad suffix"},
			{Rule: 0, Kind: refusal.ReservedIdentifier},
			{Rule: 1, Kind: refusal.BlankIdentifier},
			{Rule: 1, Kind: refusal.NoSuffix},
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
	if diff := cmp.Diff([]rules.Problem{{Rule: 0, Kind: refusal.OtherClass}}, got, compare.Options); diff != "" {
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
	s := rules.NewSenders(senders, prepared(), lookups)
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
	s := rules.NewSenders(senders, prepared(), lookups)
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

// A stored domain is classified as the classifier classifies an address at it: the rule restricting a
// sender is the first the classifier meets, a sender no rule matches is normal with none, and a domain
// the classifier cannot read is restricted with none (ADR-0004, ADR-0136).
func TestClassify(t *testing.T) {
	policy := prepared(rule("base.schwab", "schwab.com"), rule("own.mail", "mail.schwab.com"))
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
		v := rules.Classify(policy, domain)
		if got := (class{v.Class().Restricted(), v.Rule()}); got != want {
			t.Errorf("Classify(%q) = %+v, want %+v", domain, got, want)
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
			s := rules.NewSenders(senders, prepared(c.policy...), lookups)
			if diff := cmp.Diff(c.restricted, s.Restricted(), compare.Options); diff != "" {
				t.Errorf("Restricted (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.normal, s.Normal(), compare.Options); diff != "" {
				t.Errorf("Normal (-want +got):\n%s", diff)
			}
		})
	}
	if got := rules.NewSenders(nil, prepared(), lookups).Normal(); got == nil {
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

// A write is decided against the scope's rules as read, the way each refusal is answered on the wire:
// an identifier the scope holds is taken, a rule the scope does not hold is unknown before it is ever
// stale, suffixes other than those the page read are stale, a rule the checks refuse names its
// problems with the rule's identifier, and removing a base rule's suffix or lifting a base rule
// needs the rule's identifier typed (docs/UI.md sections 8.7 and 17.4, ADR-0084, VERIFICATIONS, the
// policy write rows).
func TestAWriteIsDecidedAgainstTheScopesRules(t *testing.T) {
	stored := []rules.Rule{rule("bank", "bank.example"), rule("lender", "lender.example", "loans.example")}
	typed := func(s string) *string { return &s }
	cases := []struct {
		name string
		got  *refusal.Refusal
		want *refusal.Refusal
	}{
		{"a new rule", rules.Adding(stored, rule("shop", "shop.example")), nil},
		{"a held identifier", rules.Adding(stored, rule("bank", "other.example")), rules.Taken()},
		{"a rule the checks refuse", rules.Adding(stored, rule("shop", "*.shop.example")), &refusal.Refusal{
			Code: refusal.RuleRefused, Message: "the rule fails the policy's checks",
			Problems: []refusal.Problem{{Kind: refusal.InvalidSuffix, Rule: "shop", Suffix: "*.shop.example"}},
		}},
		{"an edit", rules.Editing(stored, "bank", []string{"bank.example"}, []string{"bank.example", "card.example"}, false, nil), nil},
		{"an edit of an unknown rule with stale suffixes", rules.Editing(stored, "shop", []string{"x.example"}, []string{"y.example"}, false, nil), rules.UnknownRule()},
		{"an edit against stale suffixes", rules.Editing(stored, "bank", []string{"old.example"}, []string{"bank.example"}, false, nil), rules.StaleRule()},
		{"an edit removing every suffix", rules.Editing(stored, "bank", []string{"bank.example"}, []string{}, true, typed("bank")), &refusal.Refusal{
			Code: refusal.RuleRefused, Message: "the rule fails the policy's checks",
			Problems: []refusal.Problem{{Kind: refusal.NoSuffix, Rule: "bank"}},
		}},
		{"an account rule's edit removing a suffix", rules.Editing(stored, "lender", []string{"lender.example", "loans.example"}, []string{"lender.example"}, false, nil), nil},
		{"a base rule's edit removing a suffix unconfirmed", rules.Editing(stored, "lender", []string{"lender.example", "loans.example"}, []string{"lender.example"}, true, typed("lend")), refusal.Of(refusal.ConfirmationRequired, "lifting a base rule's restriction needs the rule's identifier typed")},
		{"a base rule's edit removing a suffix confirmed", rules.Editing(stored, "lender", []string{"lender.example", "loans.example"}, []string{"lender.example"}, true, typed(" lender ")), nil},
		{"a base rule's edit adding a suffix", rules.Editing(stored, "bank", []string{"bank.example"}, []string{"bank.example", "card.example"}, true, nil), nil},
		{"an account rule's lift", rules.Lifting(stored, "bank", []string{"bank.example"}, false, nil), nil},
		{"a lift of an unknown rule", rules.Lifting(stored, "shop", []string{"shop.example"}, true, typed("shop")), rules.UnknownRule()},
		{"a lift against stale suffixes", rules.Lifting(stored, "bank", []string{"old.example"}, true, typed("bank")), rules.StaleRule()},
		{"a base rule's lift unconfirmed", rules.Lifting(stored, "bank", []string{"bank.example"}, true, nil), refusal.Of(refusal.ConfirmationRequired, "lifting a base rule's restriction needs the rule's identifier typed")},
		{"a base rule's lift confirmed", rules.Lifting(stored, "bank", []string{"bank.example"}, true, typed("bank")), nil},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, c.got, compare.Options); diff != "" {
			t.Errorf("%s (-want +got):\n%s", c.name, diff)
		}
	}
	for _, r := range []*refusal.Refusal{rules.Taken(), rules.UnknownRule(), rules.StaleRule()} {
		if r.Message == "" {
			t.Errorf("%s carries no message", r.Code)
		}
	}
}

// The policy screens' search keeps a rule whose identifier or one of whose suffixes holds the text,
// compared without case, and an empty search keeps every rule (docs/UI.md sections 5 and 8.14).
func TestTheSearchMatchesAnIdentifierOrASuffixWithoutCase(t *testing.T) {
	r := rule("Base.Bank", "Bank.Example", "card.example")
	for search, want := range map[string]bool{"": true, "base.b": true, "BANK.EX": true, "CARD": true, "lender": false, "bank.example.": false} {
		if got := rules.Matches(r, search); got != want {
			t.Errorf("%q: %v, want %v", search, got, want)
		}
	}
}
