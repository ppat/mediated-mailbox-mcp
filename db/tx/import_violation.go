//go:build banproof

package tx

// This file breaks the data-access import list on purpose, and banproof requires the want below.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/jmap" // want depguard "list 'db'"
)
