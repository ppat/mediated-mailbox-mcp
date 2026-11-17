//go:build banproof

package session

// This file breaks the account session's import list on purpose. The session imports no provider
// adapter and no rate limiter, since each composition root passes it a connector per provider and
// leases inside each call it makes.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail" // want depguard "list 'session'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"  // want depguard "list 'session'"
)
