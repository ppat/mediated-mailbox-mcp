package classify_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// outcome is what a test can observe of a verdict.
type outcome struct {
	Restricted bool
	Reason     string
	Rule       string
}

func observe(v classify.Verdict) outcome {
	return outcome{Restricted: v.Class().Restricted(), Reason: v.Reason().String(), Rule: v.Rule()}
}

func snapshot(t *testing.T) policy.Snapshot {
	t.Helper()
	s, err := policy.Load([]policy.Row{
		{ID: "financial.brokerage.fidelity", Class: policy.Restricted, DomainSuffixes: []string{"fidelity.com", "fmr.com"}},
		{ID: "gov.federal.irs", Class: policy.Restricted, DomainSuffixes: []string{"irs.gov"}},
		{ID: "unicode.rule", Class: policy.Restricted, DomainSuffixes: []string{"München.de"}},
		{ID: "punycode.rule", Class: policy.Restricted, DomainSuffixes: []string{"xn--bcher-kva.ch"}},
		{Account: "acct-a", ID: "candidate.acct-a.examplebank.com", Class: policy.Restricted, DomainSuffixes: []string{"examplebank.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// lookups are the real functions a composition root passes in (ADR-0043).
var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

func listed(rule string) outcome { return outcome{Restricted: true, Reason: "listed", Rule: rule} }

var (
	unlisted       = outcome{Reason: "unlisted"}
	unclassifiable = outcome{Restricted: true, Reason: "unclassifiable"}
)

func TestClassify(t *testing.T) {
	s := snapshot(t)
	cases := []struct {
		name, account, address string
		want                   outcome
	}{
		{"a listed domain", "acct-a", "alerts@fidelity.com", listed("financial.brokerage.fidelity")},
		{"a subdomain of a listed domain", "acct-a", "x@alerts.fidelity.com", listed("financial.brokerage.fidelity")},
		{"a second suffix of one rule", "acct-a", "x@mail.fmr.com", listed("financial.brokerage.fidelity")},
		{"mixed case", "acct-a", "x@Alerts.FIDELITY.com", listed("financial.brokerage.fidelity")},
		{"a trailing dot", "acct-a", "x@fidelity.com.", listed("financial.brokerage.fidelity")},
		{"a sender in punycode against a rule in Unicode", "acct-a", "x@alerts.xn--mnchen-3ya.de", listed("unicode.rule")},
		{"a sender in Unicode against a rule in punycode", "acct-a", "x@bücher.ch", listed("punycode.rule")},
		{"the account's own overlay", "acct-a", "x@examplebank.com", listed("candidate.acct-a.examplebank.com")},
		{"another account's overlay", "acct-b", "x@examplebank.com", unlisted},
		{"a listed name inside a longer label", "acct-a", "x@notfidelity.com", unlisted},
		{"a listed name followed by another domain", "acct-a", "x@fidelity.com.evil.example", unlisted},
		{"an unlisted domain", "acct-a", "x@newsletter.example", unlisted},
		{"the last at sign ends the local part", "acct-a", `"a@b"@fidelity.com`, listed("financial.brokerage.fidelity")},
		{"no at sign", "acct-a", "fidelity.com", unclassifiable},
		{"no domain", "acct-a", "x@", unclassifiable},
		{"a public suffix alone", "acct-a", "x@com", unclassifiable},
		{"an address literal", "acct-a", "x@[192.0.2.1]", unclassifiable},
		{"a domain that is not a name", "acct-a", "x@exa mple.com", unclassifiable},
		{"an empty label after the trailing dot", "acct-a", "x@fidelity.com..", unclassifiable},
		{"an empty label inside", "acct-a", "x@alerts..fidelity.com", unclassifiable},
		{"a leading dot", "acct-a", "x@.fidelity.com", unclassifiable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(classify.Classify(s.For(c.account), c.address, lookups))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Classify(%q) (-want +got):\n%s", c.address, diff)
			}
		})
	}
}

// Before any valid policy has loaded, every sender is restricted, an unlisted one included
// (ADR-0041).
func TestNoPolicyRestrictsEverySender(t *testing.T) {
	var none policy.Snapshot
	got := observe(classify.Classify(none.For("acct-a"), "x@newsletter.example", lookups))
	if diff := cmp.Diff(outcome{Restricted: true, Reason: "no policy"}, got, compare.Options); diff != "" {
		t.Errorf("Classify with no policy (-want +got):\n%s", diff)
	}
}

// A verdict nobody built is the most restrictive one (ADR-0042).
func TestTheZeroVerdictIsRestricted(t *testing.T) {
	if diff := cmp.Diff(unclassifiable, observe(classify.Verdict{}), compare.Options); diff != "" {
		t.Errorf("zero Verdict (-want +got):\n%s", diff)
	}
}
