//go:build banproof

package markdown

// This file breaks the sanitization library's import list on purpose. The list for non-test code admits
// the data-access library, so only the library's own list reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/tx" // want depguard "list 'sanitize'"
)
