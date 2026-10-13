package classify_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// perCall classifies the sender of address under p converting every suffix on the call, as the
// classifier did before its policy was prepared once per load. It is the reference the prepared
// policy is compared with, an example-based comparison over listed cases, since ADR-0055 bars a
// property from checking what the classifier decides.
func perCall(p policy.Composed, address string) outcome {
	if p.RestrictsAll() {
		return outcome{Restricted: true, Reason: "no policy"}
	}
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return unclassifiable
	}
	sender, ok := classify.SenderName(address[at+1:], lookups)
	if !ok {
		return unclassifiable
	}
	for _, r := range p.Rules() {
		for _, suffix := range r.DomainSuffixes() {
			if sender.Under(classify.SuffixName(suffix, lookups)) {
				return outcome{Restricted: true, Reason: "listed", Rule: r.ID(), Scope: r.Scope().String()}
			}
		}
	}
	return unlisted
}

// The policy Prepare builds classifies every sender as converting the suffixes on every call does,
// for rules written in upper case, in full-width letters, with a dotted capital I, in upper-case
// punycode, and in punycode the lookups cannot decode, which both compare lowercased as written. The
// senders are each case of TestClassify, the shared fixtures' senders, and senders written every way
// those rules are, against the base policy and an account's, and while no policy has loaded
// (ADR-0004, ADR-0041).
func TestThePreparedPolicyClassifiesAsConvertingOnEveryCall(t *testing.T) {
	s, err := policy.Load([]policy.Row{
		{ID: "financial.brokerage.fidelity", Class: policy.Restricted, DomainSuffixes: []string{"fidelity.com", "fmr.com"}},
		{ID: "unicode.rule", Class: policy.Restricted, DomainSuffixes: []string{"München.de"}},
		{ID: "punycode.rule", Class: policy.Restricted, DomainSuffixes: []string{"xn--bcher-kva.ch"}},
		{ID: "upper.rule", Class: policy.Restricted, DomainSuffixes: []string{"MÜNCHEN.example", "BİNK.example"}},
		{ID: "fullwidth.rule", Class: policy.Restricted, DomainSuffixes: []string{"ｂａｎｋ.example"}},
		{ID: "upper.punycode.rule", Class: policy.Restricted, DomainSuffixes: []string{"XN--BCHER-KVA.example"}},
		{ID: "undecodable.rule", Class: policy.Restricted, DomainSuffixes: []string{"xn--a.example", "XN--A.Other"}},
		{ID: "fixture.rule", Class: policy.Restricted, DomainSuffixes: []string{"bank.example", "security.example"}},
		{Account: "acct-a", ID: "candidate.acct-a.examplebank.com", Class: policy.Restricted, DomainSuffixes: []string{"examplebank.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	senders := []string{
		"alerts@fidelity.com", "x@alerts.fidelity.com", "x@mail.fmr.com", "x@Alerts.FIDELITY.com", "x@fidelity.com.",
		"x@alerts.xn--mnchen-3ya.de", "x@bücher.ch", "x@examplebank.com", "x@notfidelity.com",
		"x@fidelity.com.evil.example", "x@newsletter.example", `"a@b"@fidelity.com`, "fidelity.com", "x@", "x@com",
		"x@[192.0.2.1]", "x@exa mple.com", "x@fidelity.com..", "x@alerts..fidelity.com", "x@.fidelity.com",
		"x@münchen.example", "x@MÜNCHEN.EXAMPLE", "x@xn--mnchen-3ya.example", "x@bİnk.example", "x@bink.example",
		"x@bank.example", "x@ＢＡＮＫ.example", "x@bücher.example", "x@XN--BCHER-KVA.example",
		"x@xn--a.example", "x@sub.xn--a.example", "x@xn--a.other", "x@XN--A.Other",
	}
	for _, m := range fixture.All() {
		senders = append(senders, m.FromAddress)
	}
	var none policy.Snapshot
	listedBy := map[string]bool{}
	for _, account := range []string{"acct-a", "acct-b"} {
		for name, composed := range map[string]policy.Composed{"loaded": s.For(account), "never loaded": none.For(account)} {
			prepared := classify.Prepare(composed, lookups)
			for _, sender := range senders {
				want := perCall(composed, sender)
				listedBy[want.Rule] = want.Reason == "listed" || listedBy[want.Rule]
				if diff := cmp.Diff(want, observe(classify.Classify(prepared, sender)), compare.Options); diff != "" {
					t.Errorf("%s policy of %s, Classify(%+q) (-converted on every call +prepared):\n%s", name, account, sender, diff)
				}
			}
		}
	}
	// The comparison proves nothing for a rule no sender is listed by, so every rule must list one,
	// apart from the rule the lookups cannot decode. A sender under it cannot be decoded either, so it
	// is unclassifiable and no sender is listed by that rule either way.
	for _, r := range s.For("acct-a").Rules() {
		if !listedBy[r.ID()] && r.ID() != "undecodable.rule" {
			t.Errorf("no sender is listed by %s under the policy converted on every call", r.ID())
		}
	}
}

// Classify takes the prepared policy and the address and nothing else, and the prepared policy holds
// the lookups its suffixes were converted with, so no call can normalize a sender with lookups other
// than the ones the policy was prepared with.
func TestASenderIsNormalizedWithThePoliciesOwnLookups(t *testing.T) {
	mustnotcompile.RequireParams(t, classifyPackage, "Classify", "Policy", "string")
	mustnotcompile.RequireFields(t, classifyPackage, "Policy", "composed policy.Composed", "rules []prepared", "lookups Lookups")
}

const classifyPackage = "github.com/ppat/mediated-mailbox-mcp/core/classify"
