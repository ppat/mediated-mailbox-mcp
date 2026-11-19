//go:build banproof

package process

// This file imports on purpose the packages of the fences table of ADR-0071 that the process family's
// root package may not reach, which are `credential/open`, `provider/consent`, `provider/mail/gmail`,
// `provider/mail/gmail/core`, `provider/ratelimit` and `provider/ratelimit/core`. The list 'process'
// names none of them, so it refuses each here, as the annotation on each import shows. No other list
// matching this file refuses them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'process'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'process'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'process'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'process'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'process'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'process'"
)
