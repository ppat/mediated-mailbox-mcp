//go:build banproof

package migrate

// This file imports on purpose the packages of the fences table of ADR-0071 that the application of
// the migration chain may not reach, which are `credential/open`, `provider/consent`,
// `provider/mail/gmail`, `provider/mail/gmail/core`, `provider/ratelimit` and
// `provider/ratelimit/core`. The list 'db-migrate' names none of them, so it refuses each here, as the
// annotation on each import shows. No other list matching this file refuses them, so this file is what
// proves each fence here. The list refuses `process/connections` here too, and no violation file
// proves it, because importing it from here would make an import cycle in the ban-proof build, which
// compiles every violation file.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'db-migrate'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'db-migrate'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'db-migrate'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'db-migrate'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'db-migrate'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'db-migrate'"
)
