//go:build banproof

package reads

// This file imports, from a read handler's package, each provider's consent, the sealing half of the
// credential code, which loads the key credentials are sealed to, the client-secret package, the
// setups, policy management and the pages, on purpose. The setups build the key and the consents and
// keep them, and only the read API's package mounts the setups, the reads and policy management, so
// the UI's list over non-test code outside the setups and the read API refuses the first five, and the
// reads' own list, which names no other handler package, refuses all six. So no read handler reaches
// the key, a consent, a stored secret, a setup, a policy write or a page (ADR-0081, ADR-0084).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/seal"                // want depguard "list 'ui-outside-setup'" depguard "list 'ui-reads'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"               // want depguard "list 'ui-outside-setup'" depguard "list 'ui-reads'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/pages"              // want depguard "list 'ui-reads'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/policymanagement"   // want depguard "list 'ui-outside-setup'" depguard "list 'ui-reads'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup"              // want depguard "list 'ui-outside-setup'" depguard "list 'ui-reads'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup/clientsecret" // want depguard "list 'ui-outside-setup'" depguard "list 'ui-reads'"
)
