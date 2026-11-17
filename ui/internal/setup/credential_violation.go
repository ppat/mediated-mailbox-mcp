//go:build banproof

package setup

// This file imports the credential code's opening half, and the read of a client's sealed secret, on
// purpose. The setups' list admits neither, so the setups reach a stored client's secret only through
// the one operation of ui/internal/setup/clientsecret, a package of its own, and in the UI only the
// code that sets up an OAuth client and an account opens a stored value (ADR-0081, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'ui-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/oauthclients" // want depguard "list 'ui-setup'"
)
