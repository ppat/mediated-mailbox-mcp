//go:build banproof

package app

// This file breaks the UI entry's import list on purpose. The entry holds the UI's spec and nothing
// else, so it runs no statement and builds no part: a data-access subsection, the checks every
// endpoint sits behind and the client-secret opener are each refused, while the packages whose Builds
// the spec names are admitted (ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accounts" // want depguard "list 'ui-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/pages"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/server"             // want depguard "list 'ui-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup/clientsecret" // want depguard "list 'ui-app'" depguard "list 'ui-outside-setup'"
)
