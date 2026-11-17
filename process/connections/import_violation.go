//go:build banproof

package connections

// This file breaks the connections' import list on purpose. They reach a provider only through the
// provider library's calling side, so an adapter and the rate limiter are refused (ADR-0010, ADR-0071).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail" // want depguard "list 'connections'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"  // want depguard "list 'connections'"
)
