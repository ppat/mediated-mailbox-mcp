package redact_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// masked is what a test can observe of a masked subject.
type masked struct {
	Subject string
	Events  []event
}

type event struct {
	Rule string
	Tier int
}

func observeMask(m redact.Masked) masked {
	got := masked{Subject: m.Subject()}
	for _, e := range m.Events() {
		got.Events = append(got.Events, event{Rule: e.Rule(), Tier: e.Tier()})
	}
	return got
}

func defaultScanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Without a built scanner nothing can be told apart, so the whole subject is masked, character by
// character, and the zero Masked is an empty subject (ADR-0042).
func TestMaskingFailsClosedWithoutAScanner(t *testing.T) {
	var none scan.Scanner
	got := observeMask(redact.MaskSubject(none, "Réservation 419283"))
	want := masked{Subject: "██████████████████", Events: []event{{Rule: redact.RuleWholeSubject}}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("MaskSubject without a scanner (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(masked{}, observeMask(redact.Masked{}), compare.Options); diff != "" {
		t.Errorf("zero Masked (-want +got):\n%s", diff)
	}
}

// Each detection is replaced by one █ per character, and each mask records its rule and tier, never
// the text (ADR-0003).
func TestMaskSubject(t *testing.T) {
	s := defaultScanner(t)
	window := event{Rule: scan.RuleTriggerWindow, Tier: 1}
	path := event{Rule: scan.RuleLinkPath, Tier: 1}
	cases := []struct {
		name, subject string
		want          masked
	}{
		{"a code after a trigger", "Your code is 419283", masked{Subject: "Your code is ██████", Events: []event{window}}},
		{"a code in groups", "Code: 419 283", masked{Subject: "Code: ███████", Events: []event{window}}},
		{"a code among accented letters", "Réservation code 419283", masked{Subject: "Réservation code ██████", Events: []event{window}}},
		{"an alphanumeric code", "Your sign-in code is 7GX4Q2", masked{Subject: "Your sign-in code is ██████", Events: []event{{Rule: scan.RuleScore, Tier: 2}}}},
		{"a subject with nothing to mask", "Overdraft notice, action required", masked{Subject: "Overdraft notice, action required"}},
		{"a code in groups joined by a no-break space", "Code: 419\u00a0283", masked{Subject: "Code: ███████", Events: []event{window}}},
		{"an order number", "Order 20240917 has shipped", masked{Subject: "Order 20240917 has shipped"}},
		{"an alphanumeric order number", "Order A1B2C3 has shipped", masked{Subject: "Order A1B2C3 has shipped"}},
		{"an alphanumeric code before a trigger", "7GX4Q2 - Acme sign-in", masked{Subject: "██████ - Acme sign-in", Events: []event{{Rule: scan.RuleScore, Tier: 2}}}},
		{"a link before a full stop", "Sign in at https://x.example/magic/Q7fT2kLm9ZpX4vRw.", masked{Subject: "Sign in at " + strings.Repeat("█", 40) + ".", Events: []event{path}}},
		{"a link holding an accented letter", "Go https://x.example/magic/Q7fT2kLm9ZpX4vRwé", masked{Subject: "Go " + strings.Repeat("█", 41), Events: []event{path}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observeMask(redact.MaskSubject(s, c.subject)), compare.Options); diff != "" {
				t.Errorf("MaskSubject(%q) (-want +got):\n%s", c.subject, diff)
			}
		})
	}
}

// A subject carrying a one-time code, from a sender the policy restricts, is masked. The restricted
// sender's body is never scanned, and masking still runs (ADR-0003, ADR-0008).
func TestARestrictedSendersCodeSubjectIsMasked(t *testing.T) {
	m := fixture.OneTimeCode()
	p := load(t, policy.Row{ID: "security", Class: policy.Restricted, DomainSuffixes: []string{"security.example"}})
	if !classify.Classify(p, m.FromAddress, lookups).Class().Restricted() {
		t.Fatal("the fixture's sender is not restricted under the test's policy")
	}
	got := redact.MaskSubject(defaultScanner(t), m.Subject).Subject()
	if diff := cmp.Diff(marker.Field("codesubject")+" Your code is ██████", got, compare.Options); diff != "" {
		t.Errorf("masked subject (-want +got):\n%s", diff)
	}
}

// Masking takes the scanner and the subject and nothing else, so no sender class can skip it, and
// what it returns holds the masked subject and events naming rules, never the text they masked.
func TestMaskingTakesNoSenderClass(t *testing.T) {
	mustnotcompile.RequireParams(t, redactPackage, "MaskSubject", "scan.Scanner", "string")
	mustnotcompile.RequireFields(t, redactPackage, "Masked", "subject string", "events []Event")
	mustnotcompile.RequireFields(t, redactPackage, "Event", "rule string", "tier int")
}
