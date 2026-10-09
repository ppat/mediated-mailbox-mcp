package service

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// The readers of the index's stored sensitivity read a content flag or a scan state the schema does
// not name as the most restrictive state, so a row the schema's checks would refuse is never released
// if one were ever stored (ADR-0042, ADR-0016). The schema's checks refuse such a row, so no row in a
// database reaches this, and the readers are tested on their own. The test reads unexported functions,
// so it sits in the package.
func TestAnUnnamedStoredStateReadsAsTheMostRestrictive(t *testing.T) {
	for _, names := range [][]string{{"unknown_flag"}, {"mfa_code", "unknown_flag"}, {"MFA_CODE"}} {
		if got := storedFlags(names); !got.MFACode() || !got.LoginLink() {
			t.Errorf("the stored flags %v read as mfa_code %v and login_link %v, want both", names, got.MFACode(), got.LoginLink())
		}
	}
	if got := storedFlags([]string{"mfa_code"}); !got.MFACode() || got.LoginLink() {
		t.Errorf("the stored flag mfa_code reads as mfa_code %v and login_link %v, want mfa_code alone", got.MFACode(), got.LoginLink())
	}
	for _, name := range []string{"unknown_state", "SCANNED", ""} {
		if got := storedScanState(name); got != sensitivity.Pending() {
			t.Errorf("the stored scan state %q reads as %s, want %s", name, got, sensitivity.Pending())
		}
	}
	if got := storedScanState("scanned"); got != sensitivity.Scanned() {
		t.Errorf("the stored scan state scanned reads as %s, want %s", got, sensitivity.Scanned())
	}
}
