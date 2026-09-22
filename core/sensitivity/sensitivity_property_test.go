package sensitivity_test

import (
	"errors"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// args are the constructor arguments of one message's sensitivity and body, each drawn on its own
// (ADR-0069).
type args struct {
	Restricted, MFACode, LoginLink bool
	Scan                           int
	Tag                            string
}

var scanStates = []func() sensitivity.ScanState{
	sensitivity.Pending, sensitivity.Scanned, sensitivity.SkippedGate, sensitivity.SkippedRestricted,
}

const (
	pending, scanned, skippedGate, skippedRestricted = 0, 1, 2, 3
)

func draw(t *rapid.T) args {
	return args{
		Restricted: rapid.Bool().Draw(t, "restricted"),
		MFACode:    rapid.Bool().Draw(t, "mfaCode"),
		LoginLink:  rapid.Bool().Draw(t, "loginLink"),
		Scan:       rapid.IntRange(0, len(scanStates)-1).Draw(t, "scan"),
		Tag:        rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "tag"),
	}
}

func build(a args) (sensitivity.Sensitivity, error) {
	class := sensitivity.NormalSender()
	if a.Restricted {
		class = sensitivity.RestrictedSender()
	}
	return sensitivity.New(class, sensitivity.Flags(a.MFACode, a.LoginLink), scanStates[a.Scan]())
}

// releases states ADR-0007's release rule over the drawn arguments, apart from the code. A body is
// available only for a normal sender with no content flag, scanned or skipped by the scan gate.
func releases(a args) bool {
	return !a.Restricted && !a.MFACode && !a.LoginLink && (a.Scan == scanned || a.Scan == skippedGate)
}

// A postcondition over every sensitivity a message can have. No body value carrying restricted
// sensitivity, a content flag or a denying scan state can be constructed, the committed example of
// ADR-0055, and every sensitivity the rule releases does yield the body, so a constructor refusing
// everything fails too.
func TestNoBodyCarriesADenyingSensitivity(t *testing.T) {
	property.Check(t, draw, func(t rapid.TB, a args) {
		s, err := build(a)
		if err != nil {
			return
		}
		text := marker.Body(a.Tag)
		b, err := sensitivity.NewBody(text, s)
		switch {
		case releases(a) && err != nil:
			t.Fatalf("%+v releases a body, but NewBody refused it: %v", a, err)
		case releases(a) && b.Text() != text:
			t.Fatalf("%+v released the body %q, want %q", a, b.Text(), text)
		case !releases(a) && !errors.Is(err, sensitivity.ErrBodyWithheld):
			t.Fatalf("%+v withholds its body, but NewBody returned %q with error %v", a, b.Text(), err)
		case !releases(a) && b.Text() != "":
			t.Fatalf("%+v withholds its body, but a refused Body still holds %q", a, b.Text())
		}
	})
}

// kind names what a drawn case exercises. A boundary case is one where exactly one of sender class,
// content flags and scan state withholds the body, so a single change would release it. Those are
// where a leak would hide.
func kind(a args) string {
	if _, err := build(a); err != nil {
		return "refused"
	}
	classDenies := a.Restricted
	flagsDeny := a.MFACode || a.LoginLink
	scanDenies := a.Scan == pending || a.Scan == skippedRestricted
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
// producing. Over 32 equally likely combinations the scan-state boundary is the rarest kind, at 1
// in 32, and across thirty seeds at 100 and at 200 cases it never fell below 1 percent.
func TestNoBodyCarriesADenyingSensitivityMix(t *testing.T) {
	property.Report(t, draw, kind, map[string]float64{
		"releases":                0.01,
		"boundary: sender class":  0.01,
		"boundary: content flags": 0.01,
		"boundary: scan state":    0.01,
	})
}
