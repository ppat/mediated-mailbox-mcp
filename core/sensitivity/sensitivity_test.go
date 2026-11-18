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
}

// The combinations ADR-0069 counts as occurring are accepted, and the others refused, with the
// release rule of ADR-0093 on each accepted one.
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

// No sensitivity-carrying type exposes a field, so no code outside the package can build or change
// one around its constructors, whatever fields the types gain later (ADR-0042).
func TestNoSensitivityCarryingTypeExposesAField(t *testing.T) {
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/core/sensitivity",
		"SenderClass", "ContentFlags", "ScanState", "Sensitivity")
}

// Building a sensitivity or a scan state around the constructors does not compile (ADR-0042).
func TestConstructionOutsideTheConstructorsDoesNotCompile(t *testing.T) {
	cases := []struct{ name, want string }{
		{"sensitivityliteral", "cannot refer to unexported field class in struct literal of type sensitivity.Sensitivity"},
		{"scanstateliteral", "cannot refer to unexported field state in struct literal of type sensitivity.ScanState"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustnotcompile.Require(t, "./testdata/mustnotcompile/"+c.name, c.want)
		})
	}
}

// The readers of the index's stored sensitivity read a content flag or a scan state the schema does
// not name as the most restrictive state, so a row the schema's checks would refuse is never released
// if one were ever stored (ADR-0042, ADR-0016). The schema's checks refuse such a row, so no row in a
// database reaches this, and the readers are tested on their own.
func TestAnUnnamedStoredStateReadsAsTheMostRestrictive(t *testing.T) {
	for _, names := range [][]string{{"unknown_flag"}, {"mfa_code", "unknown_flag"}, {"MFA_CODE"}} {
		if got := sensitivity.FlagsNamed(names); !got.MFACode() || !got.LoginLink() {
			t.Errorf("the stored flags %v read as mfa_code %v and login_link %v, want both", names, got.MFACode(), got.LoginLink())
		}
	}
	if got := sensitivity.FlagsNamed([]string{"mfa_code"}); !got.MFACode() || got.LoginLink() {
		t.Errorf("the stored flag mfa_code reads as mfa_code %v and login_link %v, want mfa_code alone", got.MFACode(), got.LoginLink())
	}
	for _, name := range []string{"unknown_state", "SCANNED", ""} {
		if got := sensitivity.ScanStateNamed(name); got != sensitivity.Pending() {
			t.Errorf("the stored scan state %q reads as %s, want %s", name, got, sensitivity.Pending())
		}
	}
	if got := sensitivity.ScanStateNamed("scanned"); got != sensitivity.Scanned() {
		t.Errorf("the stored scan state scanned reads as %s, want %s", got, sensitivity.Scanned())
	}
}

// Each stored name reads back as the value that writes it, so the index's writers and readers agree
// on every name, and the names are the ones the schema's checks hold (ADR-0016).
func TestTheStoredNamesReadBackAsWritten(t *testing.T) {
	type flagsCase struct {
		MFACode, LoginLink bool
		Names              []string
	}
	var gotFlags []flagsCase
	for _, f := range []sensitivity.ContentFlags{sensitivity.NoFlags(), sensitivity.Flags(true, false), sensitivity.Flags(false, true), sensitivity.Flags(true, true)} {
		back := sensitivity.FlagsNamed(f.Names())
		gotFlags = append(gotFlags, flagsCase{MFACode: back.MFACode(), LoginLink: back.LoginLink(), Names: f.Names()})
	}
	wantFlags := []flagsCase{
		{Names: []string{}},
		{MFACode: true, Names: []string{"mfa_code"}},
		{LoginLink: true, Names: []string{"login_link"}},
		{MFACode: true, LoginLink: true, Names: []string{"mfa_code", "login_link"}},
	}
	if diff := cmp.Diff(wantFlags, gotFlags, compare.Options); diff != "" {
		t.Errorf("content flags (-want +got):\n%s", diff)
	}
	var gotStates []string
	for _, s := range []sensitivity.ScanState{sensitivity.Pending(), sensitivity.Scanned(), sensitivity.SkippedGate(), sensitivity.SkippedRestricted(), sensitivity.Refused()} {
		if back := sensitivity.ScanStateNamed(s.String()); back != s {
			t.Errorf("the scan state %s reads back as %s", s, back)
		}
		gotStates = append(gotStates, s.String())
	}
	if diff := cmp.Diff([]string{"pending", "scanned", "skipped_gate", "skipped_restricted", "refused"}, gotStates); diff != "" {
		t.Errorf("scan state names (-want +got):\n%s", diff)
	}
	type classCase struct {
		Name       string
		Restricted bool
		Named      bool
	}
	var gotClasses []classCase
	for _, name := range []string{"normal", "restricted", "Normal", ""} {
		c, named := sensitivity.SenderClassNamed(name)
		gotClasses = append(gotClasses, classCase{Name: c.String(), Restricted: c.Restricted(), Named: named})
	}
	wantClasses := []classCase{
		{Name: "normal", Named: true},
		{Name: "restricted", Restricted: true, Named: true},
		{Name: "restricted", Restricted: true},
		{Name: "restricted", Restricted: true},
	}
	if diff := cmp.Diff(wantClasses, gotClasses, compare.Options); diff != "" {
		t.Errorf("sender classes (-want +got):\n%s", diff)
	}
}
