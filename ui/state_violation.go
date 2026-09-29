//go:build banproof

package main

// This file imports the account state's credential subsection and the rate state's limiter
// subsection on purpose. Of those two tables the component's list admits only the parent subsections
// one directory up, so the UI's role is never planned against a statement that touches a credential
// or writes the rate state (ADR-0084, ADR-0091).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential" // want depguard "list 'ui'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/ratestate/limiter"       // want depguard "list 'ui'"
)
