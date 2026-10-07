//go:build banproof

package session

// This file breaks the account session's import list on purpose. The session imports no provider
// adapter and no rate limiter, since each composition root passes it a connector per provider and
// leases inside each call it makes.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/gmail"  // want depguard "list 'accountload-session'"
	_ "github.com/ppat/mediated-mailbox-mcp/ratelimit/lease" // want depguard "list 'accountload-session'"
)
