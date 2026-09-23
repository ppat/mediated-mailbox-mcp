package redact_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// outcome is what a test can observe of a verdict.
type outcome struct {
	Reason                                           string
	ReleasesBody, ShowsSnippet, ShowsAttachmentNames bool
	Sender                                           string
}

func observe(v redact.Verdict) outcome {
	return outcome{
		Reason:               v.Reason().String(),
		ReleasesBody:         v.ReleasesBody(),
		ShowsSnippet:         v.ShowsSnippet(),
		ShowsAttachmentNames: v.ShowsAttachmentNames(),
		Sender:               v.Sender().Reason().String(),
	}
}

// lookups are the real functions a composition root passes in (ADR-0043).
var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

const (
	listed   = "alerts@fidelity.com"
	unlisted = "sam@example.com"
)

func load(t *testing.T, rows ...policy.Row) policy.Composed {
	t.Helper()
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return s.For("acct-a")
}

var fidelity = policy.Row{ID: "financial.brokerage.fidelity", Class: policy.Restricted, DomainSuffixes: []string{"fidelity.com"}}

func denied(reason, sender string) outcome { return outcome{Reason: reason, Sender: sender} }

var released = outcome{Reason: "released", ReleasesBody: true, ShowsSnippet: true, ShowsAttachmentNames: true, Sender: "unlisted"}

var (
	none    = sensitivity.NoFlags()
	mfa     = sensitivity.Flags(true, false)
	link    = sensitivity.Flags(false, true)
	scanned = sensitivity.Scanned()
)

// The paths production never exercises, tested first (ADR-0042). Each denies.
func TestFailClosed(t *testing.T) {
	var never policy.Snapshot
	p := load(t, fidelity)
	cases := []struct {
		name   string
		p      policy.Composed
		sender string
		flags  sensitivity.ContentFlags
		scan   sensitivity.ScanState
		want   outcome
	}{
		{"a policy that never loaded", never.For("acct-a"), unlisted, none, scanned, denied("restricted sender", "no policy")},
		{"a sender the classifier cannot read", p, "no-at-sign", none, scanned, denied("restricted sender", "unclassifiable")},
		{"a sender with no registrable domain", p, "x@com", none, scanned, denied("restricted sender", "unclassifiable")},
		{"an unscanned message", p, unlisted, none, sensitivity.Pending(), denied("pending content scan", "unlisted")},
		{"stored values nobody built", p, unlisted, sensitivity.ContentFlags{}, sensitivity.ScanState{}, denied("invalid stored state", "unlisted")},
		{"a flag on a message skipped by the scan gate", p, unlisted, link, sensitivity.SkippedGate(), denied("invalid stored state", "unlisted")},
		{"a flag on a message skipped as restricted", p, listed, mfa, sensitivity.SkippedRestricted(), denied("invalid stored state", "listed")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(redact.Decide(c.p, c.sender, lookups, c.flags, c.scan))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Decide (-want +got):\n%s", diff)
			}
		})
	}
}

// A verdict nobody built withholds the body as an invalid stored state (ADR-0042).
func TestTheZeroVerdictWithholds(t *testing.T) {
	if diff := cmp.Diff(denied("invalid stored state", "unclassifiable"), observe(redact.Verdict{}), compare.Options); diff != "" {
		t.Errorf("zero Verdict (-want +got):\n%s", diff)
	}
}

// ADR-0001's matrix, one case per column and per scan state, with ADR-0002's deny branches.
func TestDecide(t *testing.T) {
	p := load(t, fidelity)
	cases := []struct {
		name   string
		sender string
		flags  sensitivity.ContentFlags
		scan   sensitivity.ScanState
		want   outcome
	}{
		{"a normal sender, scanned clean", unlisted, none, scanned, released},
		{"a normal sender, skipped by the scan gate", unlisted, none, sensitivity.SkippedGate(), released},
		{"a restricted sender, skipped as restricted", listed, none, sensitivity.SkippedRestricted(), denied("skipped as restricted", "listed")},
		{"a restricted sender, scanned before it was listed", listed, none, scanned, denied("restricted sender", "listed")},
		{"a restricted sender, skipped by the scan gate", listed, none, sensitivity.SkippedGate(), denied("restricted sender", "listed")},
		{"a restricted sender, pending", listed, none, sensitivity.Pending(), denied("pending content scan", "listed")},
		{"a one-time code", unlisted, mfa, scanned, denied("content flagged", "unlisted")},
		{"a login link", unlisted, link, scanned, denied("content flagged", "unlisted")},
		{"both flags", unlisted, sensitivity.Flags(true, true), scanned, denied("content flagged", "unlisted")},
		{"a restricted sender with a flag", listed, mfa, scanned, denied("restricted sender", "listed")},
		{"a delisted sender before its messages return to pending", unlisted, none, sensitivity.SkippedRestricted(), denied("skipped as restricted", "unlisted")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := observe(redact.Decide(p, c.sender, lookups, c.flags, c.scan))
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Decide (-want +got):\n%s", diff)
			}
		})
	}
}

// The stored state is the same before and after the domain is listed, with no re-sync in between,
// and the decision follows the policy it is given (ADR-0002).
func TestAJustListedDomainDeniesOnTheNextDecision(t *testing.T) {
	before := load(t)
	after := load(t, policy.Row{ID: "just.listed", Class: policy.Restricted, DomainSuffixes: []string{"example.com"}})
	got := []outcome{
		observe(redact.Decide(before, unlisted, lookups, none, scanned)),
		observe(redact.Decide(after, unlisted, lookups, none, scanned)),
	}
	want := []outcome{released, denied("restricted sender", "listed")}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("before and after listing (-want +got):\n%s", diff)
	}
}

const (
	redactPackage      = "github.com/ppat/mediated-mailbox-mcp/core/redact"
	classifyPackage    = "github.com/ppat/mediated-mailbox-mcp/core/classify"
	policyPackage      = "github.com/ppat/mediated-mailbox-mcp/core/policy"
	sensitivityPackage = "github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// A verdict cannot carry a body, a snippet or a filename. It holds a reason and the sender's
// classification, which holds a class, a reason and a rule identifier, and nothing else. A literal
// giving it a body does not compile (ADR-0040).
func TestTheVerdictCannotCarryContent(t *testing.T) {
	mustnotcompile.RequireFields(t, redactPackage, "Verdict", "reason Reason", "sender classify.Verdict")
	mustnotcompile.RequireFields(t, classifyPackage, "Verdict", "class sensitivity.SenderClass", "reason Reason", "rule string")
	mustnotcompile.RequireFields(t, sensitivityPackage, "SenderClass", "normal bool")
	mustnotcompile.Require(t, "./testdata/mustnotcompile/verdictbody", "unknown field body in struct literal of type redact.Verdict")
}

// The decision takes the policy, the sender, the classifier's lookups and the stored flags and scan
// state, and nothing a provider or a body could arrive through, so a denial never contacts the
// provider (ADR-0002). Each struct among them is checked down to its last field, because a provider
// method would arrive as a function value inside one.
func TestTheDecisionTakesNoProviderAndNoBody(t *testing.T) {
	mustnotcompile.RequireParams(t, redactPackage, "Decide",
		"policy.Composed", "string", "classify.Lookups", "sensitivity.ContentFlags", "sensitivity.ScanState")
	mustnotcompile.RequireFields(t, policyPackage, "Composed", "loaded bool", "rules []Rule")
	mustnotcompile.RequireFields(t, policyPackage, "Rule", "id string", "suffixes []string")
	mustnotcompile.RequireFields(t, classifyPackage, "Lookups",
		"ToUnicode func(domain string) (string, error)",
		"ToASCII func(domain string) (string, error)",
		"Registrable func(domain string) (string, error)")
	mustnotcompile.RequireFields(t, sensitivityPackage, "ContentFlags", "noMFACode bool", "noLoginLink bool")
	mustnotcompile.RequireFields(t, sensitivityPackage, "ScanState", "state scanState")
}
