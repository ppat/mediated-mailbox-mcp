package sensitivity_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// view is what a test can observe of a Sensitivity through its accessors.
type view struct {
	Restricted, MFACode, LoginLink bool
	Scan                           string
	ReleasesBody                   bool
}

func observe(s sensitivity.Sensitivity) view {
	return view{
		Restricted:   s.Class().Restricted(),
		MFACode:      s.Flags().MFACode(),
		LoginLink:    s.Flags().LoginLink(),
		Scan:         s.Scan().String(),
		ReleasesBody: s.ReleasesBody(),
	}
}

func TestZeroValuesAreTheMostRestrictive(t *testing.T) {
	var s sensitivity.Sensitivity
	want := view{Restricted: true, MFACode: true, LoginLink: true, Scan: "pending", ReleasesBody: false}
	if diff := cmp.Diff(want, observe(s), compare.Options); diff != "" {
		t.Errorf("zero Sensitivity (-want +got):\n%s", diff)
	}
	var b sensitivity.Body
	if diff := cmp.Diff("", b.Text(), compare.Options); diff != "" {
		t.Errorf("zero Body text (-want +got):\n%s", diff)
	}
}

// The combinations ADR-0069 counts as occurring are accepted, and the others refused, with the
// release rule of ADR-0007 on each accepted one.
func TestNew(t *testing.T) {
	normal, restricted := sensitivity.NormalSender(), sensitivity.RestrictedSender()
	none, mfa, link, both := sensitivity.NoFlags(), sensitivity.Flags(true, false), sensitivity.Flags(false, true), sensitivity.Flags(true, true)
	pending, scanned, gate, skipped := sensitivity.Pending(), sensitivity.Scanned(), sensitivity.SkippedGate(), sensitivity.SkippedRestricted()

	type outcome struct {
		View view
		Err  error
	}
	cases := []struct {
		name  string
		class sensitivity.SenderClass
		flags sensitivity.ContentFlags
		scan  sensitivity.ScanState
		want  outcome
	}{
		{"normal, no flags, scanned", normal, none, scanned, outcome{View: view{Scan: "scanned", ReleasesBody: true}}},
		{"normal, no flags, skipped by the gate", normal, none, gate, outcome{View: view{Scan: "skipped_gate", ReleasesBody: true}}},
		{"normal, no flags, pending", normal, none, pending, outcome{View: view{Scan: "pending"}}},
		{"normal, code, scanned", normal, mfa, scanned, outcome{View: view{MFACode: true, Scan: "scanned"}}},
		{"normal, link, scanned", normal, link, scanned, outcome{View: view{LoginLink: true, Scan: "scanned"}}},
		{"normal, both, scanned", normal, both, scanned, outcome{View: view{MFACode: true, LoginLink: true, Scan: "scanned"}}},
		{"restricted, no flags, scanned", restricted, none, scanned, outcome{View: view{Restricted: true, Scan: "scanned"}}},
		{"restricted, code, scanned", restricted, mfa, scanned, outcome{View: view{Restricted: true, MFACode: true, Scan: "scanned"}}},
		{"restricted, no flags, skipped by the gate", restricted, none, gate, outcome{View: view{Restricted: true, Scan: "skipped_gate"}}},
		{"restricted, no flags, skipped as restricted", restricted, none, skipped, outcome{View: view{Restricted: true, Scan: "skipped_restricted"}}},
		{"restricted, no flags, pending", restricted, none, pending, outcome{View: view{Restricted: true, Scan: "pending"}}},
		{"normal, code, pending", normal, mfa, pending, outcome{Err: sensitivity.ErrFlagsWithoutScan}},
		{"normal, link, skipped by the gate", normal, link, gate, outcome{Err: sensitivity.ErrFlagsWithoutScan}},
		{"restricted, both, skipped as restricted", restricted, both, skipped, outcome{Err: sensitivity.ErrFlagsWithoutScan}},
		{"normal, no flags, skipped as restricted", normal, none, skipped, outcome{Err: sensitivity.ErrNormalSkippedRestricted}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := sensitivity.New(c.class, c.flags, c.scan)
			got := outcome{Err: err}
			if err == nil {
				got.View = observe(s)
			}
			if diff := cmp.Diff(c.want, got, compare.Options, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("New (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewBody(t *testing.T) {
	must := func(class sensitivity.SenderClass, flags sensitivity.ContentFlags, scan sensitivity.ScanState) sensitivity.Sensitivity {
		t.Helper()
		s, err := sensitivity.New(class, flags, scan)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	type outcome struct {
		Text string
		Err  error
	}
	cases := []struct {
		name string
		s    sensitivity.Sensitivity
		want outcome
	}{
		{"scanned", must(sensitivity.NormalSender(), sensitivity.NoFlags(), sensitivity.Scanned()), outcome{Text: "released"}},
		{"skipped by the gate", must(sensitivity.NormalSender(), sensitivity.NoFlags(), sensitivity.SkippedGate()), outcome{Text: "released"}},
		{"restricted sender", must(sensitivity.RestrictedSender(), sensitivity.NoFlags(), sensitivity.Scanned()), outcome{Err: sensitivity.ErrBodyWithheld}},
		{"one-time code", must(sensitivity.NormalSender(), sensitivity.Flags(true, false), sensitivity.Scanned()), outcome{Err: sensitivity.ErrBodyWithheld}},
		{"login link", must(sensitivity.NormalSender(), sensitivity.Flags(false, true), sensitivity.Scanned()), outcome{Err: sensitivity.ErrBodyWithheld}},
		{"pending scan", must(sensitivity.NormalSender(), sensitivity.NoFlags(), sensitivity.Pending()), outcome{Err: sensitivity.ErrBodyWithheld}},
		{"skipped as restricted", must(sensitivity.RestrictedSender(), sensitivity.NoFlags(), sensitivity.SkippedRestricted()), outcome{Err: sensitivity.ErrBodyWithheld}},
		{"never constructed", sensitivity.Sensitivity{}, outcome{Err: sensitivity.ErrBodyWithheld}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := sensitivity.NewBody("released", c.s)
			got := outcome{Text: b.Text(), Err: err}
			if diff := cmp.Diff(c.want, got, compare.Options, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("NewBody (-want +got):\n%s", diff)
			}
		})
	}
}

// No sensitivity-carrying type exposes a field, so no code outside the package can build or change
// one around its constructors, whatever fields the types gain later (ADR-0042).
func TestNoSensitivityCarryingTypeExposesAField(t *testing.T) {
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/core/sensitivity",
		"SenderClass", "ContentFlags", "ScanState", "Sensitivity", "Body")
}

// Building a body, or a sensitivity, around the constructor does not compile (ADR-0042).
func TestConstructionOutsideTheConstructorsDoesNotCompile(t *testing.T) {
	cases := []struct{ name, want string }{
		{"bodyliteral", "cannot refer to unexported field text in struct literal of type sensitivity.Body"},
		{"sensitivityliteral", "cannot refer to unexported field class in struct literal of type sensitivity.Sensitivity"},
		{"scanstateliteral", "cannot refer to unexported field state in struct literal of type sensitivity.ScanState"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustnotcompile.Require(t, "./testdata/mustnotcompile/"+c.name, c.want)
		})
	}
}
