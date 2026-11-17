//go:build banproof

package app

// This file breaks the API server entry's import list on purpose. The entry holds the API server's
// spec and nothing else, so it runs no statement and builds no part: a data-access subsection, the
// Session, the connections, the registry's build and the body conversion are each refused, while the
// two roots, whose Build the spec names, are admitted (ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/api"
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/registry" // want depguard "list 'api-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown"      // want depguard "list 'api'" depguard "list 'api-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/messages"           // want depguard "list 'api-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"   // want depguard "list 'api'" depguard "list 'api-app'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/sessions"      // want depguard "list 'api'" depguard "list 'api-app'"
)
