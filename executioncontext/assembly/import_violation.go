//go:build banproof

package assembly

// This file breaks the execution context's import list on purpose. The context imports no provider
// adapter, since each component passes its own connectors, and a third-party package no list over
// shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'assembly'" depguard "list 'non-test-code'"

	_ "github.com/ppat/mediated-mailbox-mcp/provider/gmail" // want depguard "list 'assembly'"
)
