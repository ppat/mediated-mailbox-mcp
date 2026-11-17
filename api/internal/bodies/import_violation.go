//go:build banproof

package bodies

// This file reaches the rate limiter from the body operation's provider functions on purpose. Only the
// registry's build reaches it, for the rate-state collector, and the body operation spends its rate
// budget through its caller (ADR-0117, ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit" // want depguard "list 'api-bodies'"
)
