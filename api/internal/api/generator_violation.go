//go:build banproof

package api

// This file imports on purpose the packages of the fences table of ADR-0071 that the API root's
// generator may not reach, which are `credential/open`, `process/connections`, `provider/consent`,
// `provider/mail/gmail`, `provider/mail/gmail/core`, `provider/ratelimit` and
// `provider/ratelimit/core`. The list 'api-api-generator' names none of them, so it refuses each here,
// as the annotation on each import shows. The annotation on each import names every list that refuses
// it here. The generator's own list matches this file beside the generator, so it proves what that
// list refuses, which no violation file could while the list matched the generator alone.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'api'" depguard "list 'api-api-generator'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'api'" depguard "list 'api-api-generator'"
)
