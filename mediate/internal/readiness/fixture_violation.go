//go:build banproof

package readiness

// This file imports test tooling and the provider fake into code that ships, on purpose. The
// marker definitions are imported too and must not be reported, because the readiness probe builds
// its fixture from them.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/core/marker"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/fake"       // want depguard "list 'non-test-code'"
	_ "github.com/ppat/mediated-mailbox-mcp/testsupport/fixture" // want depguard "list 'non-test-code'"
)
