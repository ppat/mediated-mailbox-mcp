package authorize_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// args are what the authorizer decides from, each drawn on its own (ADR-0069). Verb runs past the
// last verb so values no constant names are drawn too.
type args struct {
	Verb       uint8
	Restricted bool
}

func draw(t *rapid.T) args {
	return args{
		Verb:       rapid.Uint8Range(0, uint8(authorize.Mute)+3).Draw(t, "verb"),
		Restricted: rapid.Bool().Draw(t, "restricted"),
	}
}

// organize and dispose are ADR-0019's two groups of verbs. A restricted sender's mail may only be
// organized.
var (
	organize = map[authorize.Verb]bool{authorize.Label: true, authorize.Unlabel: true, authorize.Move: true, authorize.MarkRead: true, authorize.Star: true}
	dispose  = map[authorize.Verb]bool{authorize.Archive: true, authorize.Trash: true, authorize.Spam: true, authorize.Mute: true}
)

// authorizes states ADR-0019's rules over the drawn arguments, apart from the code.
func authorizes(a args) bool {
	verb := authorize.Verb(a.Verb)
	return organize[verb] || (dispose[verb] && !a.Restricted)
}

// A postcondition over every verb value and sender class. The authorizer allows exactly what the
// rules allow, so an authorizer allowing too much fails and so does one allowing nothing.
func TestTheAuthorizerAllowsOnlyWhatTheRulesAllow(t *testing.T) {
	property.Check(t, draw, func(t rapid.TB, a args) {
		class := sensitivity.NormalSender()
		if a.Restricted {
			class = sensitivity.RestrictedSender()
		}
		v := authorize.Decide(authorize.Verb(a.Verb), class)
		if v.Authorized() != authorizes(a) {
			t.Fatalf("%+v: authorized %v with reason %q, want %v", a, v.Authorized(), v.Reason(), authorizes(a))
		}
	})
}

// kind names what a drawn case exercises. A restricted sender asked to dispose is the boundary where
// a leak would hide, and a normal sender asked the same is its pair.
func kind(a args) string {
	verb := authorize.Verb(a.Verb)
	switch {
	case organize[verb]:
		return "organize"
	case dispose[verb] && a.Restricted:
		return "dispose, restricted"
	case dispose[verb]:
		return "dispose, normal"
	default:
		return "unknown verb"
	}
}

// The generator report for the property above. Each minimum catches a kind the generator stops
// producing. rapid draws small values more often, so the kinds are not equally likely, and across
// thirty seeds at 200 cases none fell below 1 percent.
func TestTheAuthorizerAllowsOnlyWhatTheRulesAllowMix(t *testing.T) {
	property.Report(t, draw, kind, map[string]float64{
		"organize":            0.01,
		"dispose, restricted": 0.01,
		"dispose, normal":     0.01,
		"unknown verb":        0.01,
	})
}
