package livecontract_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

// A live test runs only when the marker names its provider. Without it, or with another provider's,
// the test skips before anything after the guard runs.
func TestRequire(t *testing.T) {
	cases := []struct {
		name   string
		marker string
		set    bool
		runs   bool
	}{
		{"no marker", "", false, false},
		{"an empty marker", "", true, false},
		{"another provider's marker", "gcal", true, false},
		{"a marker in another case", "Gmail", true, false},
		{"its provider's marker", "gmail", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(livecontract.Marker, c.marker)
			if !c.set {
				if err := os.Unsetenv(livecontract.Marker); err != nil {
					t.Fatal(err)
				}
			}
			ran := false
			t.Run("live", func(t *testing.T) {
				livecontract.Require(t, "gmail")
				ran = true
			})
			if ran != c.runs {
				t.Errorf("the live test ran past the guard %v, want %v", ran, c.runs)
			}
		})
	}
}

// The command runs the provider's one live test under its build tag, and sets the marker to the
// provider, replacing any the caller set, with the caller's environment passed through.
func TestInvocation(t *testing.T) {
	credential := "GMAIL" + "_TEST_REFRESH_TOKEN=1//token"
	got, err := livecontract.Invocation("gmail", []string{"PATH=/bin", credential, livecontract.Marker + "=gcal"})
	if err != nil {
		t.Fatalf("Invocation: %v", err)
	}
	want := livecontract.Command{
		Args: []string{"test", "-tags", "gmail_live", "-count=1", "-timeout", "55m", "-v", "-run", "^TestTheAdapterPassesTheContractAgainstGmail$", "./provider/gmail/"},
		Env:  []string{"PATH=/bin", credential, livecontract.Marker + "=gmail"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Invocation (-want +got):\n%s", diff)
	}
}

// A provider no live run exists for is refused, and nothing is run.
func TestInvocationRefusesAnUnknownProvider(t *testing.T) {
	for _, provider := range []string{"", "gcal", "GMAIL", "gmail ", "../gmail"} {
		got, err := livecontract.Invocation(provider, []string{"PATH=/bin"})
		if err == nil {
			t.Errorf("Invocation(%q) returned %+v, want it refused", provider, got)
		}
		if err != nil && !strings.Contains(err.Error(), "gmail") {
			t.Errorf("Invocation(%q)'s refusal %q names no provider a run exists for", provider, err)
		}
		if slices.ContainsFunc(got.Env, func(v string) bool { return strings.HasPrefix(v, livecontract.Marker) }) {
			t.Errorf("Invocation(%q) set the marker", provider)
		}
	}
}
