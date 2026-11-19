//go:build banproof

package caldav

// This file imports on purpose the packages of the fences table of ADR-0071 that the provider
// library's packages outside the calling side, the Gmail adapter and the rate limiter may not reach,
// which are `credential/open`, `process/connections` and `provider/consent`. The list 'provider' names
// none of them, so it refuses each here, as the annotation on each import shows. No other list
// matching this file refuses them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"     // want depguard "list 'provider'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections" // want depguard "list 'provider'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"    // want depguard "list 'provider'"
)
