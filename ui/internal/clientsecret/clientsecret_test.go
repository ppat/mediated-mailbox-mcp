package clientsecret_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// The package's one operation takes a client's name, never a sealing context, so no caller can ask it
// to open a value bound to an account credential, and its opener exposes no field to reach the keyring
// through (ADR-0081, VERIFICATIONS, the UI's open of a credential row).
func TestTheOpenerTakesNoContext(t *testing.T) {
	mustnotcompile.Require(t, "./testdata/mustnotcompile/opensacredential",
		`cannot use seal.AccountCredential("personal") (value of struct type seal.Context) as string value in argument to opener.Secret`)
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret", "Opener")
}
