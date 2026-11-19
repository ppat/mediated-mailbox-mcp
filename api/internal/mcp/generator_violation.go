//go:build banproof

package mcp

// This file imports on purpose the packages of the fences table of ADR-0071 that the MCP root's
// generator may not reach, which are `credential/open`, `process/connections`, `provider/consent`,
// `provider/mail/gmail`, `provider/mail/gmail/core`, `provider/ratelimit` and
// `provider/ratelimit/core`. The list 'api-mcp-generator' names none of them, so it refuses each here,
// as the annotation on each import shows. No other list matching this file refuses them, so this file
// is what proves each fence here. The generator's own list matches this file beside the generator, so
// it proves what that list refuses, which no violation file could while the list matched the generator
// alone.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'api-mcp-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'api-mcp-generator'"
)
