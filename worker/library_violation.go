//go:build banproof

package main

// This file imports the body conversion, the shared libraries that run statements or hold user state,
// the opening code, the process family's packages the worker's entry has no use for, and the Gmail
// adapter's pure core on purpose. The worker's entry lists its job kinds and builds nothing, so the
// worker's own list admits none of them, while the list for non-test code admits each, so only the
// worker's list reports them. Its job kinds' lists admit what each kind needs (ADR-0142).
import (
	_ "github.com/JohannesKaufmann/html-to-markdown/v2/converter" // want depguard "list 'worker'"

	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown"         // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/sessions"         // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'worker'"
)
