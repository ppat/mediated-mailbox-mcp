//go:build banproof

package classify_test

// This file imports rapid and its test support into an ordinary test file on purpose.
import (
	_ "pgregory.net/rapid" // want depguard "list 'ordinary-tests'"

	_ "github.com/ppat/mediated-mailbox-mcp/testsupport/property" // want depguard "list 'ordinary-tests'"
)
