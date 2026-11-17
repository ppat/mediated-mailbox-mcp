//go:build banproof

package app

// This file breaks the migrator entry's import list on purpose. The entry holds the migrator's spec
// and nothing else, so it runs no statement and applies no chain itself: a data-access subsection,
// db/migrate and goose are each refused, while the task's package, which the spec names, is admitted
// (ADR-0142).
import (
	_ "github.com/pressly/goose/v3" // want depguard "list 'migrator'"

	_ "github.com/ppat/mediated-mailbox-mcp/db/accounts" // want depguard "list 'migrator'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/migrate"  // want depguard "list 'migrator'"
	_ "github.com/ppat/mediated-mailbox-mcp/migrator/internal/migrate"
)
