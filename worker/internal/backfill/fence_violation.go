//go:build banproof

package backfill

// This file imports on purpose the packages of the fences table of ADR-0071 that backfill's job kind
// may not reach, which are `provider/consent` and `provider/mail/gmail/core`. The list 'worker-
// backfill' names none of them, so it refuses each here, as the annotation on each import shows. No
// other list matching this file refuses them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'worker-backfill'"
)
