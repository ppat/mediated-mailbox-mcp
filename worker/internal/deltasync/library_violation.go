//go:build banproof

package deltasync

// This file imports the opening code, an adapter, the rate limiter and the rate limiter's pure rules on
// purpose. A job kind takes the connections from the process family and calls the provider through a
// caller of its own and a unit of work, so it opens no credential, names no adapter and builds no
// limiter, and only the provider library's calling side reaches them (ADR-0071, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"         // want depguard "list 'worker-sync'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"     // want depguard "list 'worker-sync'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"      // want depguard "list 'worker-sync'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core" // want depguard "list 'worker-sync'"
)
