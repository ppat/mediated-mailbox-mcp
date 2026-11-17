//go:build banproof

package policies

// This file breaks the policies' import list on purpose. The opening half of the credential family,
// which the process family admits to process/connections alone, is refused, and so is a data-access
// package the list does not name (ADR-0071).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'policies'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/check"        // want depguard "list 'policies'"
)
