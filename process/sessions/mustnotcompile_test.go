package sessions_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// F9's part of VERIFICATIONS' row for a configuration type that lacks a part, for the Session. The
// UI's configuration type has no accounts part, so the compiler refuses its asking for the Session
// (ADR-0142).
func TestTheUICannotAskForTheSession(t *testing.T) {
	mustnotcompile.Require(t, "./testdata/mustnotcompile/getfromui",
		"config.UI does not implement config.HasAccounts (missing method AccountsPart)")
}
