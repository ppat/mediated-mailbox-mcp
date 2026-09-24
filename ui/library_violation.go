//go:build banproof

package main

// This file imports a provider adapter, the rate limiter and the body conversion on purpose. The
// component's list admits none of them, while the list for non-test code admits all three, so only the
// component's list reports them.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/gmail"    // want depguard "list 'ui'"
	_ "github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"   // want depguard "list 'ui'"
	_ "github.com/ppat/mediated-mailbox-mcp/sanitize/markdown" // want depguard "list 'ui'"
)
