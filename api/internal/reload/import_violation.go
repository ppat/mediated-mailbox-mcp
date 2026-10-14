//go:build banproof

package reload

// This file imports test tooling and the provider fake into code that ships, on purpose.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/provider/fake"       // want depguard "list 'api-no-http'" depguard "list 'non-test-code'"
	_ "github.com/ppat/mediated-mailbox-mcp/testsupport/fixture" // want depguard "list 'api-no-http'" depguard "list 'non-test-code'"
)
