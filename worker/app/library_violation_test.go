//go:build banproof

package app_test

// This file imports the opening code, an adapter and the rate limiter into the worker's test files on
// purpose. The worker's tests run it through its spec and hand the process the provider fake through
// the connections' one option for a test, so they open no credential, name no adapter and build no
// limiter, and the worker's test files' list admits none of them, while the list over every test file
// admits each, so only the worker's test files' list reports them (ADR-0071, ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"     // want depguard "list 'worker-tests'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail" // want depguard "list 'worker-tests'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"  // want depguard "list 'worker-tests'"
)
