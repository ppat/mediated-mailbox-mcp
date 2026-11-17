//go:build banproof

package reads

// This file imports, from a read handler's package, each provider's consent, the sealing half of the
// credential code, which loads the key credentials are sealed to, and the client-secret package, on
// purpose. The setups build the key and the consents and keep them, and only the setups' list admits
// the consents, so the UI's list refuses them here, and its list over non-test code outside the setups
// refuses all three, so no read handler reaches the key, a consent or a stored secret (ADR-0081,
// ADR-0084).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/seal"                // want depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"               // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup/clientsecret" // want depguard "list 'ui-outside-setup'"
)
