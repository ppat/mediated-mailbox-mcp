package connections_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// F9's part of VERIFICATIONS' row for a configuration type that lacks a part, for the connections. The
// UI's configuration type holds the credential part and no accounts part, so the compiler refuses its
// asking for the connections (ADR-0142).
func TestTheUICannotAskForTheConnections(t *testing.T) {
	mustnotcompile.Require(t, "./testdata/mustnotcompile/offromui",
		"config.UI does not implement config.HasConnections (missing method AccountsPart)")
}
