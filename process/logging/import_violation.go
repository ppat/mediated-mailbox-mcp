//go:build banproof

package logging

// This file breaks the logger's import list on purpose. The logger writes records and touches no
// database. The list for non-test code admits the data-access library, so only the logger's own list
// reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/tx" // want depguard "list 'logging'"
)
