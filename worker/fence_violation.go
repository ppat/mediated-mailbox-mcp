//go:build banproof

package main

// This file imports on purpose the packages of the fences table of ADR-0071 that the worker's entry
// and the series its job kinds share may not reach, which are `provider/consent`,
// `provider/mail/gmail` and `provider/ratelimit/core`. The list 'worker' names none of them, so it
// refuses each here, as the annotation on each import shows. No other list matching this file refuses
// them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"        // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"     // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core" // want depguard "list 'worker'"
)
