//go:build banproof

package accounts

// This file breaks the reading of the list's import list on purpose. The list holds no credential, so
// the opening half of the credential family, which the process family admits to process/connections
// alone, is refused here, and so is the subsection of the stored credentials (ADR-0071, ADR-0091).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"            // want depguard "list 'accounts'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential" // want depguard "list 'accounts'"
)
