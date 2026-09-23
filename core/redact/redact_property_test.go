package redact_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// args are what the gate decides from, each drawn on its own (ADR-0069). Listed says whether the
// sender's domain is on the policy, and the flags and scan state are what the index stores.
type args struct {
	Listed, MFACode, LoginLink bool
	Scan                       int
}

var scanStates = []func() sensitivity.ScanState{
	sensitivity.Pending, sensitivity.Scanned, sensitivity.SkippedGate, sensitivity.SkippedRestricted,
}

const (
	pending, scannedState, skippedGate, skippedRestricted = 0, 1, 2, 3
)

func draw(t *rapid.T) args {
	return args{
		Listed:    rapid.Bool().Draw(t, "listed"),
		MFACode:   rapid.Bool().Draw(t, "mfaCode"),
		LoginLink: rapid.Bool().Draw(t, "loginLink"),
		Scan:      rapid.IntRange(0, len(scanStates)-1).Draw(t, "scan"),
	}
}

var generated = func() policy.Composed {
	s, err := policy.Load([]policy.Row{fidelity})
	if err != nil {
		panic(err)
	}
	return s.For("acct-a")
}()

func decide(a args) redact.Verdict {
	sender := unlisted
	if a.Listed {
		sender = listed
	}
	return redact.Decide(generated, sender, lookups, sensitivity.Flags(a.MFACode, a.LoginLink), scanStates[a.Scan]())
}

// releases states ADR-0002's release rule over the drawn arguments, apart from the code. A body is
// released only for a sender the policy does not list, with no content flag, scanned or skipped by
// the scan gate.
func releases(a args) bool {
	return !a.Listed && !a.MFACode && !a.LoginLink && (a.Scan == scannedState || a.Scan == skippedGate)
}

// A postcondition over every stored state, for a listed and an unlisted sender under a loaded
// policy. The gate releases exactly when the rule does, and the snippet and attachment filenames
// follow the body (ADR-0001), so a gate releasing too much fails and so does one releasing nothing.
func TestTheGateReleasesOnlyWhatTheRuleReleases(t *testing.T) {
	property.Check(t, draw, func(t rapid.TB, a args) {
		v := decide(a)
		want := releases(a)
		if v.ReleasesBody() != want || v.ShowsSnippet() != want || v.ShowsAttachmentNames() != want {
			t.Fatalf("%+v: body %v, snippet %v, filenames %v, reason %q, want all %v",
				a, v.ReleasesBody(), v.ShowsSnippet(), v.ShowsAttachmentNames(), v.Reason(), want)
		}
	})
}

// kind names what a drawn case exercises. A boundary case is one where exactly one of the sender,
// the content flags and the scan state withholds the body, so a single change would release it.
// Those are where a leak would hide.
func kind(a args) string {
	flagsDeny := a.MFACode || a.LoginLink
	if flagsDeny && a.Scan != scannedState {
		return "invalid"
	}
	scanDenies := a.Scan == pending || a.Scan == skippedRestricted
	switch {
	case !a.Listed && !flagsDeny && !scanDenies:
		return "releases"
	case a.Listed && !flagsDeny && !scanDenies:
		return "boundary: sender"
	case !a.Listed && flagsDeny && !scanDenies:
		return "boundary: content flags"
	case !a.Listed && !flagsDeny && scanDenies:
		return "boundary: scan state"
	default:
		return "withheld on more than one axis"
	}
}

// The generator report for the property above. Each minimum catches a kind the generator stops
// producing. Over 32 equally likely combinations the rarest kinds are 2 in 32, and across thirty
// seeds at 200 cases none fell below 1 percent.
func TestTheGateReleasesOnlyWhatTheRuleReleasesMix(t *testing.T) {
	property.Report(t, draw, kind, map[string]float64{
		"releases":                0.01,
		"boundary: sender":        0.01,
		"boundary: content flags": 0.01,
		"boundary: scan state":    0.01,
		"invalid":                 0.01,
	})
}
