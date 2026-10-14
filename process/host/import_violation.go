//go:build banproof

package host

// This file breaks the host's import list on purpose. The host runs a process over the components it
// is handed and runs no statement of its own. The list for non-test code admits the data-access
// library, so only the host's own list reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/tx" // want depguard "list 'host'"
)
