//go:build banproof

package main

// This file imports a data-access package the component's list does not name, on purpose. The checks
// in db/check are named by no component's list.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/check" // want depguard "list 'mediate'"
)
