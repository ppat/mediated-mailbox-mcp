//go:build banproof

package main

// This file imports on purpose the packages of the fences table of ADR-0071 that the UI's code outside
// its setups and its read API may not reach, which are `process/connections` and
// `provider/ratelimit/core`. The list 'ui-outside-setup' names none of them, so it refuses each here,
// as the annotation on each import shows. No other list matching this file refuses them, so this file
// is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"     // want depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core" // want depguard "list 'ui-outside-setup'"
)
