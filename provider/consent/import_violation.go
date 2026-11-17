//go:build banproof

package consent

// This file imports the Gmail adapter, its pure core, the rate limiter and the limiter's pure core
// on purpose. Only the provider library's calling side may import the adapter and the limiter, only
// the adapter its core, and only the limiter and the calling side the limiter's, so the provider
// library's list over everything but the calling side, the Gmail adapter and the rate limiter names
// none of them (ADR-0071).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'provider'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'provider'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'provider'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'provider'"
)
