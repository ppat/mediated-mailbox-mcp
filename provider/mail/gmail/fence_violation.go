//go:build banproof

package gmail

// This file imports on purpose the packages of the fences table of ADR-0071 that the Gmail adapter may
// not reach, which are `provider/ratelimit` and `provider/ratelimit/core`. The list 'provider-gmail'
// names none of them, so it refuses each here, as the annotation on each import shows. No other list
// matching this file refuses them, so this file is what proves each fence here. The list refuses
// `credential/open` and `provider/consent` here too, and no violation file proves it, because
// importing them from here would make an import cycle in the ban-proof build, which compiles every
// violation file.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"      // want depguard "list 'provider-gmail'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core" // want depguard "list 'provider-gmail'"
)
