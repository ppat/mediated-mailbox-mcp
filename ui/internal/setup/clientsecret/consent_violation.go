//go:build banproof

package clientsecret

// This file imports each provider's consent from the opener on purpose. The opener's own list admits
// only what opening a client's secret needs, the opening half, the sealing half for the purpose bound
// into a value and the read of a client's sealed secret, so it refuses the consents, and the list
// governs every file under ui/internal/setup/clientsecret (ADR-0081, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent" // want depguard "list 'ui-clientsecret'"
)
