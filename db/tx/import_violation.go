//go:build banproof

package tx

// This file breaks the data-access import list on purpose, and banproof requires the wants below.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/jmap" // want depguard "list 'db'" depguard "list 'non-test-code'"
)
