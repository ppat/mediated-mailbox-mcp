//go:build banproof

package main

// This file imports the body conversion on purpose. The component's list does not admit it, while the
// list for non-test code does, so only the component's list reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/sanitize/markdown" // want depguard "list 'organize'"
)
