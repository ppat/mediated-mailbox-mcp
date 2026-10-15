//go:build banproof

package assembly

// This file imports a data-access subsection the execution context's list does not name, on purpose.
// A credential is written back through the account snapshot's statements, never the context's own.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential" // want depguard "list 'assembly'"
)
