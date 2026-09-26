//go:build banproof

package readiness

// This file imports test tooling and the provider fake into code that ships, on purpose.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/fake"       // want depguard "list 'mediate-no-http'" depguard "list 'non-test-code'"
	_ "github.com/ppat/mediated-mailbox-mcp/testsupport/fixture" // want depguard "list 'mediate-no-http'" depguard "list 'non-test-code'"
)
