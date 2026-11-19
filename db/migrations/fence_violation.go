//go:build banproof

package migrations

// This file imports on purpose the packages of the fences table of ADR-0071 that the data-access
// library may not reach, which are `credential/open`, `provider/consent`, `provider/mail/gmail`,
// `provider/mail/gmail/core`, `provider/ratelimit` and `provider/ratelimit/core`. The list 'db' names
// none of them, so it refuses each here, as the annotation on each import shows. No other list
// matching this file refuses them, so this file is what proves each fence here. The list refuses
// `process/connections` here too, which the compiler holds, since the connections import this package
// through process/database. A data-access subsection would hold no such cycle, and holds only
// generated files, which db/check requires.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'db'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'db'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'db'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'db'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'db'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'db'"
)
