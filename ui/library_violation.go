//go:build banproof

package main

// This file imports a provider adapter, the adapter's pure core, the rate limiter and the body
// conversion on purpose. The component's lists admit none of them, while the list for non-test code
// admits all four, so only the component's lists report them.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown"         // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
)
