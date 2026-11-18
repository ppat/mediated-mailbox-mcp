package index_test

import (
	"strconv"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
)

// rules returns the account's policy with one rule per suffix, named rule.<n> in order.
func rules(t *testing.T, suffixes ...string) classify.Policy {
	t.Helper()
	var rows []policy.Row
	for i, s := range suffixes {
		rows = append(rows, policy.Row{ID: "rule." + strconv.Itoa(i), Class: policy.Restricted, DomainSuffixes: []string{s}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return classify.Prepare(s.For(account), lookups)
}

// A sender's class is decided from the domain the index stores for it as it is from its address, for
// an address with no @, with several, with capitals, with a dotted capital I, written in full-width
// letters or in upper-case punycode, under rules written in either case. So the reads that classify
// senders by their stored domain, the sender class term, the sender listing, the delisting transition
// and the UI's classification on read, decide each sender as the gate decides it (ADR-0108, ADR-0037,
// ADR-0136).
func TestAStoredDomainClassifiesAsItsAddress(t *testing.T) {
	// The first rule is written with a dotted capital I, which a rule listing the plain i does not match.
	listed := rules(t, "B\u0130NK.example", "Mixed.Example", "b\u00fccher.example")
	plain := rules(t, "bink.example")
	type sender struct {
		address string
		domain  string
		under   map[string]bool
	}
	senders := []sender{
		{"alerts@b\u0130nk.example", "bi\u0307nk.example", map[string]bool{"listed": true, "plain": false}},
		{"ALERTS@B\u0130NK.EXAMPLE", "bi\u0307nk.example", map[string]bool{"listed": true, "plain": false}},
		{"x@sub.B\u0130nk.example", "sub.bi\u0307nk.example", map[string]bool{"listed": true, "plain": false}},
		{"a@bink.example", "bink.example", map[string]bool{"listed": false, "plain": true}},
		{"a@BINK.example", "bink.example", map[string]bool{"listed": false, "plain": true}},
		{"a@b@MIXED.example", "mixed.example", map[string]bool{"listed": true, "plain": false}},
		{"a@\uff2d\uff29\uff38\uff25\uff24.example", "\uff4d\uff49\uff58\uff45\uff44.example", map[string]bool{"listed": true, "plain": false}},
		{"a@XN--BCHER-KVA.example", "xn--bcher-kva.example", map[string]bool{"listed": true, "plain": false}},
		{"a@news.example", "news.example", map[string]bool{"listed": false, "plain": false}},
		{"MIXED.EXAMPLE", "", map[string]bool{"listed": true, "plain": true}},
		{"a@", "", map[string]bool{"listed": true, "plain": true}},
	}
	var items []mail.MessageMetadata
	for i, s := range senders {
		items = append(items, mail.MessageMetadata{AccountID: account, ID: strconv.Itoa(i), From: mail.Address{Email: s.address}})
	}
	for name, p := range map[string]classify.Policy{"listed": listed, "plain": plain} {
		page := index.Decide(items, p, scanner(t))
		var restricted []string
		for i, m := range page.Messages {
			s := senders[i]
			byAddress := classify.Classify(p, s.address).Class().Restricted()
			if m.Domain != s.domain || byAddress != s.under[name] {
				t.Errorf("under the %s policy %+q is stored as %+q and restricted %t, want %+q and %t", name, s.address, m.Domain, byAddress, s.domain, s.under[name])
			}
			byDomain := classify.Classify(p, "@"+m.Domain).Class().Restricted()
			if byDomain != s.under[name] {
				t.Errorf("under the %s policy the stored domain %+q of %+q is restricted %t, want %t as its address", name, m.Domain, s.address, byDomain, s.under[name])
			}
			if s.under[name] {
				restricted = append(restricted, m.Domain)
			}
		}
		// The delisting transition reopens none of the domains the policy restricts.
		if got := index.Delisted(p, restricted); len(got) != 0 {
			t.Errorf("under the %s policy Delisted reopens %+q", name, got)
		}
	}
}
