package authorize

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A reason no case names, as a variant added later and left unhandled would be, takes the default
// branch and refuses the mutation (ADR-0042). It needs unexported access, because no exported path
// builds a verdict around Decide.
func TestAnUnhandledReasonRefuses(t *testing.T) {
	v := Verdict{reason: Authorized + 1}
	got := []any{v.Authorized(), v.Reason().String()}
	want := []any{false, "unknown verb"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("an unhandled reason (-want +got):\n%s", diff)
	}
}
