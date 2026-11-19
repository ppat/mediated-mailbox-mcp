//go:build banproof

package main

// This file imports on purpose the packages of the fences table of ADR-0071 that the key pair check
// may not reach, which are `process/connections`, `provider/consent`, `provider/mail/fake`,
// `provider/mail/gmail`, `provider/mail/gmail/core`, `provider/ratelimit` and
// `provider/ratelimit/core`. The list 'keypaircheck' names none of them, so it refuses each here, as
// the annotation on each import shows. No other list matching this file refuses them, so this file is
// what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"       // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'keypaircheck'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'keypaircheck'"
)
