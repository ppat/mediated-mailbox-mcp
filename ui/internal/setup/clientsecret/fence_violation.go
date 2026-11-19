//go:build banproof

package clientsecret

// This file imports on purpose the packages of the fences table of ADR-0071 that the UI's client-
// secret part may not reach, which are `process/connections`, `provider/mail/gmail`,
// `provider/mail/gmail/core`, `provider/ratelimit` and `provider/ratelimit/core`. The list 'ui-
// clientsecret' names none of them, so it refuses each here, as the annotation on each import shows.
// No other list matching this file refuses them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'ui-clientsecret'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'ui-clientsecret'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'ui-clientsecret'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'ui-clientsecret'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'ui-clientsecret'"
)
