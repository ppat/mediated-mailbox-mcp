package index_test

import (
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The stored form of a domain is its lowercase by Go's simple mapping with the dotted capital I taken
// to i followed by U+0307, where the simple mapping alone gives a plain i (ADR-0016).
func TestStoredDomain(t *testing.T) {
	for in, want := range map[string]string{
		"":                         "",
		"bank.example":             "bank.example",
		"Alerts.BANK.Example":      "alerts.bank.example",
		"B\u0130NK.example":        "bi\u0307nk.example",
		"XN--BCHER-KVA.example":    "xn--bcher-kva.example",
		"B\u00dcCHER.example":      "b\u00fccher.example",
		"\uff22\uff21\uff2e\uff2b": "\uff42\uff41\uff4e\uff4b",
	} {
		if got := index.StoredDomain(in); got != want {
			t.Errorf("StoredDomain(%+q) = %+q, want %+q", in, got, want)
		}
	}
}

// rules returns the account's policy with one rule per suffix, named rule.<n> in order.
func rules(t *testing.T, suffixes ...string) policy.Composed {
	t.Helper()
	var rows []policy.Row
	for i, s := range suffixes {
		rows = append(rows, policy.Row{ID: "rule." + strconv.Itoa(i), Class: policy.Restricted, DomainSuffixes: []string{s}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return s.For(account)
}

// A sender's class is decided from the domain the index stores for it as it is from its address, for
// an address with no @, with several, with capitals, with a dotted capital I, written in full-width
// letters or in upper-case punycode, under rules written in either case. So the reads that classify
// senders by their stored domain, the sender class term, the sender listing and the delisting and
// listing transitions, decide each sender as the gate decides it (ADR-0108, ADR-0037, ADR-0113).
func TestAStoredDomainClassifiesAsItsAddress(t *testing.T) {
	// The first rule is written with a dotted capital I, which a rule listing the plain i does not match.
	listed := rules(t, "B\u0130NK.example", "Mixed.Example", "b\u00fccher.example")
	plain := rules(t, "bink.example")
	type sender struct {
		address string
		domain  string
		under   map[string]index.Class
	}
	r, n := index.Restricted, index.Normal
	senders := []sender{
		{"alerts@b\u0130nk.example", "bi\u0307nk.example", map[string]index.Class{"listed": r, "plain": n}},
		{"ALERTS@B\u0130NK.EXAMPLE", "bi\u0307nk.example", map[string]index.Class{"listed": r, "plain": n}},
		{"x@sub.B\u0130nk.example", "sub.bi\u0307nk.example", map[string]index.Class{"listed": r, "plain": n}},
		{"a@bink.example", "bink.example", map[string]index.Class{"listed": n, "plain": r}},
		{"a@BINK.example", "bink.example", map[string]index.Class{"listed": n, "plain": r}},
		{"a@b@MIXED.example", "mixed.example", map[string]index.Class{"listed": r, "plain": n}},
		{"a@\uff2d\uff29\uff38\uff25\uff24.example", "\uff4d\uff49\uff58\uff45\uff44.example", map[string]index.Class{"listed": r, "plain": n}},
		{"a@XN--BCHER-KVA.example", "xn--bcher-kva.example", map[string]index.Class{"listed": r, "plain": n}},
		{"a@news.example", "news.example", map[string]index.Class{"listed": n, "plain": n}},
		{"MIXED.EXAMPLE", "", map[string]index.Class{"listed": r, "plain": r}},
		{"a@", "", map[string]index.Class{"listed": r, "plain": r}},
	}
	var items []mail.MessageMetadata
	for i, s := range senders {
		items = append(items, mail.MessageMetadata{AccountID: account, ID: strconv.Itoa(i), From: mail.Address{Email: s.address}})
	}
	for name, p := range map[string]policy.Composed{"listed": listed, "plain": plain} {
		page := index.Decide(items, p, scanner(t), lookups)
		var restricted, normal []string
		for i, m := range page.Messages {
			s := senders[i]
			if m.Domain != s.domain || m.Class != s.under[name] {
				t.Errorf("under the %s policy %+q is stored as %+q, %s, want %+q, %s", name, s.address, m.Domain, m.Class, s.domain, s.under[name])
			}
			byDomain := index.Normal
			if classify.Classify(p, "@"+m.Domain, lookups).Class().Restricted() {
				byDomain = index.Restricted
			}
			if byDomain != s.under[name] {
				t.Errorf("under the %s policy the stored domain %+q of %+q classifies %s, want %s as its address", name, m.Domain, s.address, byDomain, s.under[name])
			}
			if s.under[name] == index.Restricted {
				restricted = append(restricted, m.Domain)
			} else {
				normal = append(normal, m.Domain)
			}
		}
		// The delisting transition reopens none of the domains the policy restricts, and the listing
		// transition restricts none of those it leaves normal.
		if got := index.Delisted(p, lookups, restricted); len(got) != 0 {
			t.Errorf("under the %s policy Delisted reopens %+q", name, got)
		}
		if got := index.Listed(p, lookups, normal); len(got) != 0 {
			t.Errorf("under the %s policy Listed restricts %+q", name, got)
		}
	}
	// Stored as normal under no rule, the domains the first policy lists are listed by it, each under its rule.
	var stored []string
	for _, m := range index.Decide(items, rules(t), scanner(t), lookups).Messages {
		if m.Class == index.Normal {
			stored = append(stored, m.Domain)
		}
	}
	want := []index.Listing{
		{Domain: "bi\u0307nk.example", Rule: "rule.0"},
		{Domain: "bi\u0307nk.example", Rule: "rule.0"},
		{Domain: "sub.bi\u0307nk.example", Rule: "rule.0"},
		{Domain: "mixed.example", Rule: "rule.1"},
		{Domain: "\uff4d\uff49\uff58\uff45\uff44.example", Rule: "rule.1"},
		{Domain: "xn--bcher-kva.example", Rule: "rule.2"},
	}
	if diff := cmp.Diff(want, index.Listed(listed, lookups, stored), compare.Options); diff != "" {
		t.Errorf("Listed over the domains stored as normal (-want +got):\n%s", diff)
	}
}

// For every code point, a domain holding it and the domain's stored form are one name to the sender
// classifier, so the lowercase mapping StoredDomain applies agrees with the classifier's own for each,
// and a change to either that splits them for any code point fails here.
func TestEveryCodePointStoresAsTheClassifierReadsIt(t *testing.T) {
	checked := 0
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		for _, d := range []string{"x" + string(r) + ".example", "example." + string(r)} {
			name, ok := classify.SenderName(d, lookups)
			storedName, storedOK := classify.SenderName(index.StoredDomain(d), lookups)
			if name != storedName || ok != storedOK {
				t.Errorf("%+q reads as %v (%v) and its stored form %+q as %v (%v)", d, name, ok, index.StoredDomain(d), storedName, storedOK)
			}
			if ok {
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no domain was readable, so nothing was compared")
	}
}
