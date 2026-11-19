//go:build banproof

package main

// This file imports the account state's credential subsection and the rate state's limiter
// subsection on purpose. Of those two tables the component's lists admit only the parent subsections
// one directory up, so the UI's code reaches no statement that touches a credential or writes the
// rate state, which the application user it connects as is granted for other programs' code (ADR-0084,
// ADR-0091, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential" // want depguard "list 'ui-outside-setup'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/ratestate/limiter"       // want depguard "list 'ui-outside-setup'"
)
