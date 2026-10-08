//go:build banproof

package main

// This file imports the credential code's opening half, and the read of a client's sealed secret, on
// purpose. The component's list admits both only in ui/internal/clientsecret, the one part of the UI
// that opens a stored value, while the list for non-test code admits both everywhere, so only the
// component's list reports them, and no other code of the UI can open a credential (ADR-0081).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/oauthclients"                  // want depguard "list 'ui'"
	_ "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open" // want depguard "list 'ui'"
)
