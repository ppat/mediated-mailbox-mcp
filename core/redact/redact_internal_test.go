package redact

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A reason no case names, as a variant added later and left unhandled would be, takes the default
// branch and withholds the body (ADR-0042). It needs unexported access, because no exported path
// builds a verdict around Decide.
func TestAnUnhandledReasonWithholds(t *testing.T) {
	v := Verdict{reason: Released + 1}
	got := []any{v.ReleasesBody(), v.ShowsSnippet(), v.ShowsAttachmentNames(), v.Reason().String()}
	want := []any{false, false, false, "invalid stored state"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("an unhandled reason (-want +got):\n%s", diff)
	}
}
