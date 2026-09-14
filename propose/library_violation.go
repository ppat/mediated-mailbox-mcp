//go:build banproof

package main

// This file imports a provider adapter and the rate limiter on purpose. The component's list admits
// neither, while the list for non-test code admits both, so only the component's list reports them.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/gmail"  // want depguard "list 'propose'"
	_ "github.com/ppat/mediated-mailbox-mcp/ratelimit/lease" // want depguard "list 'propose'"
)
