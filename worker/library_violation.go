//go:build banproof

package main

// This file imports the body conversion, the shared libraries that run statements and the Gmail
// adapter's pure core on purpose. The worker's own list admits none of them, while the list for
// non-test code admits each, so only the worker's list reports them. Its job kinds' lists admit what
// each kind needs. The worker's list admits the driver only so the composition root can open each
// job kind's pool, and a statement the worker's own code runs on a pool is left to review.
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'worker'"

	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown"             // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload" // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/executioncontext/session"     // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core"     // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"           // want depguard "list 'worker'"
)
