//go:build banproof

package fixture

// This file breaks the test tooling's import lists on purpose.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/ratelimit/lease" // want depguard "list 'testsupport'" depguard "list 'testsupport-without-rapid'"
)
