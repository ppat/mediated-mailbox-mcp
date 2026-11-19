//go:build banproof

package ratelimit

// This file imports on purpose the package of the fences table of ADR-0071 that the rate limiter may
// not reach, `provider/mail/gmail/core`. The list 'ratelimit' does not name it, so it refuses it here,
// as the annotation on the import shows. No other list matching this file refuses it, so this file is
// what proves the fence here. The list refuses `credential/open`, `provider/consent` and
// `provider/mail/gmail` here too, and no violation file proves it, because importing them from here
// would make an import cycle in the ban-proof build, which compiles every violation file.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'ratelimit'"
)
