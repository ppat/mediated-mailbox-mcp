//go:build banproof

package domains

// This file breaks the domain lookups' import list on purpose. The opening half of the credential
// family, which the process family admits to process/connections alone, is refused (ADR-0071).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'domains'"
)
