//go:build banproof

package main

// This file imports another deployable's package on purpose.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/mediate/importtarget" // want depguard "list 'propose'"
)
