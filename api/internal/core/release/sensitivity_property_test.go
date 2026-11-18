package release_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/release"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// sensitivityArgs are the constructor arguments of one message's sensitivity and body, each drawn on
// its own (ADR-0069).
type sensitivityArgs struct {
	Restricted, MFACode, LoginLink bool
	Scan                           int
	Tag                            string
}

var scanStates = []func() sensitivity.ScanState{
	sensitivity.Pending, sensitivity.Scanned, sensitivity.SkippedGate, sensitivity.SkippedRestricted, sensitivity.Refused,
}

const (
	pending, scanned, skippedGate, skippedRestricted, refused = 0, 1, 2, 3, 4
)

func drawSensitivity(t *rapid.T) sensitivityArgs {
	return sensitivityArgs{
		Restricted: rapid.Bool().Draw(t, "restricted"),
		MFACode:    rapid.Bool().Draw(t, "mfaCode"),
		LoginLink:  rapid.Bool().Draw(t, "loginLink"),
		Scan:       rapid.IntRange(0, len(scanStates)-1).Draw(t, "scan"),
		Tag:        rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "tag"),
	}
}

func buildSensitivity(a sensitivityArgs) (sensitivity.Sensitivity, error) {
	class := sensitivity.NormalSender()
	if a.Restricted {
		class = sensitivity.RestrictedSender()
	}
	return sensitivity.New(class, sensitivity.Flags(a.MFACode, a.LoginLink), scanStates[a.Scan]())
}

// releases states ADR-0093's release rule over the drawn arguments, apart from the code. A body is
// available only for a normal sender with no content flag, scanned or skipped by the scan gate.
func releases(a sensitivityArgs) bool {
	return !a.Restricted && !a.MFACode && !a.LoginLink && (a.Scan == scanned || a.Scan == skippedGate)
}

// A postcondition over every sensitivity a message can have. The release step builds no body value
// carrying restricted sensitivity, a content flag or a denying scan state, the committed example of
// ADR-0055, and every sensitivity the rule releases does yield the body, so a release step refusing
// everything fails too.
func TestNoBodyCarriesADenyingSensitivity(t *testing.T) {
	sc, err := scan.New(config.DefaultScanner(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	property.Check(t, drawSensitivity, func(t rapid.TB, a sensitivityArgs) {
		s, err := buildSensitivity(a)
		if err != nil {
			return
		}
		text := marker.Body(a.Tag)
		v, b := release.Decide(release.Content{Body: text}, s, sc)
		switch {
		case releases(a) && !v.Releases():
			t.Fatalf("%+v releases a body, but the release step withheld it as %q", a, v.Reason())
		case releases(a) && b.Text() != wrapped(text):
			t.Fatalf("%+v released the body %q, want %q", a, b.Text(), wrapped(text))
		case !releases(a) && (v.Releases() || v.Reason().String() != "scan_state_not_releasable"):
			t.Fatalf("%+v withholds its body, but the release step decided %q", a, v.Reason())
		case !releases(a) && b.Text() != "":
			t.Fatalf("%+v withholds its body, but a withheld Body still holds %q", a, b.Text())
		}
	})
}

// sensitivityKind names what a drawn case exercises. A boundary case is one where exactly one of sender class,
// content flags and scan state withholds the body, so a single change would release it. Those are
// where a leak would hide.
func sensitivityKind(a sensitivityArgs) string {
	if _, err := buildSensitivity(a); err != nil {
		return "refused"
	}
	classDenies := a.Restricted
	flagsDeny := a.MFACode || a.LoginLink
	scanDenies := a.Scan == pending || a.Scan == skippedRestricted || a.Scan == refused
	switch {
	case !classDenies && !flagsDeny && !scanDenies:
		return "releases"
	case classDenies && !flagsDeny && !scanDenies:
		return "boundary: sender class"
	case !classDenies && flagsDeny && !scanDenies:
		return "boundary: content flags"
	case !classDenies && !flagsDeny && scanDenies:
		return "boundary: scan state"
	default:
		return "withheld on more than one axis"
	}
}

// The generator report for the property above. Each minimum catches a kind the generator stops
// producing. Over 40 equally likely combinations the rarest kinds are the release and the boundaries
// of the sender class and the scan state, and across thirty seeds at 100 and at 200 cases none fell
// below 2 percent.
func TestNoBodyCarriesADenyingSensitivityMix(t *testing.T) {
	property.Report(t, drawSensitivity, sensitivityKind, map[string]float64{
		"releases":                0.01,
		"boundary: sender class":  0.01,
		"boundary: content flags": 0.01,
		"boundary: scan state":    0.01,
	})
}
