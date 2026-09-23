package authorize_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// outcome is what a test can observe of a verdict.
type outcome struct {
	Authorized bool
	Reason     string
}

func observe(v authorize.Verdict) outcome {
	return outcome{Authorized: v.Authorized(), Reason: v.Reason().String()}
}

var (
	authorized = outcome{Authorized: true, Reason: "authorized"}
	restricted = outcome{Reason: "restricted sender"}
	unknown    = outcome{Reason: "unknown verb"}
)

// The paths production never exercises, tested first (ADR-0042). Each refuses.
func TestFailClosed(t *testing.T) {
	var zeroVerb authorize.Verb
	var zeroClass sensitivity.SenderClass
	cases := []struct {
		name  string
		verb  authorize.Verb
		class sensitivity.SenderClass
		want  outcome
	}{
		{"the zero verb from a normal sender", zeroVerb, sensitivity.NormalSender(), unknown},
		{"the zero verb from a restricted sender", zeroVerb, sensitivity.RestrictedSender(), unknown},
		{"a value past the last verb", authorize.Mute + 1, sensitivity.NormalSender(), unknown},
		{"the largest value", authorize.Verb(255), sensitivity.NormalSender(), unknown},
		{"archive under a sender class nobody built", authorize.Archive, zeroClass, restricted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(authorize.Decide(c.verb, c.class)), compare.Options); diff != "" {
				t.Errorf("Decide (-want +got):\n%s", diff)
			}
		})
	}
}

// A verdict nobody built refuses the mutation (ADR-0042).
func TestTheZeroVerdictRefuses(t *testing.T) {
	if diff := cmp.Diff(unknown, observe(authorize.Verdict{}), compare.Options); diff != "" {
		t.Errorf("zero Verdict (-want +got):\n%s", diff)
	}
}

// ADR-0019's matrix, every verb against both sender classes. The content-flagged column holds by
// the decision taking no content flags, which TestTheDecisionTakesOnlyTheVerbAndTheSenderClass
// proves.
func TestMatrix(t *testing.T) {
	cases := []struct {
		verb               authorize.Verb
		normal, restricted outcome
	}{
		{authorize.Label, authorized, authorized},
		{authorize.Unlabel, authorized, authorized},
		{authorize.Move, authorized, authorized},
		{authorize.MarkRead, authorized, authorized},
		{authorize.Star, authorized, authorized},
		{authorize.Archive, authorized, restricted},
		{authorize.Trash, authorized, restricted},
		{authorize.Spam, authorized, restricted},
		{authorize.Mute, authorized, restricted},
	}
	for _, c := range cases {
		t.Run(c.verb.String(), func(t *testing.T) {
			got := []outcome{
				observe(authorize.Decide(c.verb, sensitivity.NormalSender())),
				observe(authorize.Decide(c.verb, sensitivity.RestrictedSender())),
			}
			if diff := cmp.Diff([]outcome{c.normal, c.restricted}, got, compare.Options); diff != "" {
				t.Errorf("Decide(%s) for a normal and a restricted sender (-want +got):\n%s", c.verb, diff)
			}
		})
	}
}

// Every value a Verb can hold, under a normal sender, a restricted sender and a sender class nobody
// built. Only the matrix's verbs are ever authorized, so a verb added later, permanent delete among
// them, is refused until the matrix names it (ADR-0019).
func TestOnlyTheMatrixVerbsAreAuthorized(t *testing.T) {
	var zeroClass sensitivity.SenderClass
	classes := []struct {
		name  string
		class sensitivity.SenderClass
	}{
		{"normal", sensitivity.NormalSender()},
		{"restricted", sensitivity.RestrictedSender()},
		{"nobody built", zeroClass},
	}
	got := map[string][]string{}
	for _, c := range classes {
		for v := range 256 {
			verb := authorize.Verb(v)
			if authorize.Decide(verb, c.class).Authorized() {
				got[c.name] = append(got[c.name], verb.String())
			}
		}
	}
	organize := []string{"label", "unlabel", "move", "mark_read", "star"}
	want := map[string][]string{
		"normal":       append(append([]string{}, organize...), "archive", "trash", "spam", "mute"),
		"restricted":   organize,
		"nobody built": organize,
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("authorized verbs by sender class (-want +got):\n%s", diff)
	}
}

// Every value a Verb can hold has a name, and only a verb of the matrix has its own.
func TestVerbNames(t *testing.T) {
	named := []string{"no verb", "label", "unlabel", "move", "mark_read", "star", "archive", "trash", "spam", "mute"}
	var got, want []string
	for v := range 256 {
		got = append(got, authorize.Verb(v).String())
		if v < len(named) {
			want = append(want, named[v])
		} else {
			want = append(want, "no verb")
		}
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("verb names (-want +got):\n%s", diff)
	}
}

const authorizePackage = "github.com/ppat/mediated-mailbox-mcp/core/authorize"

// The decision takes the verb and the sender class and nothing else, so content flags cannot change
// a mutation right, and the verdict holds its reason alone (ADR-0019, ADR-0040).
func TestTheDecisionTakesOnlyTheVerbAndTheSenderClass(t *testing.T) {
	mustnotcompile.RequireParams(t, authorizePackage, "Decide", "Verb", "sensitivity.SenderClass")
	mustnotcompile.RequireFields(t, authorizePackage, "Verdict", "reason Reason")
}
