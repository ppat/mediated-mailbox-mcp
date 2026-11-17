//go:build banproof

package fake

// This file breaks the import list for the fake and the contract suite on purpose.
import (
	_ "pgregory.net/rapid" // want depguard "import 'pgregory.net/rapid' is not allowed from list 'provider-test-code'"
)
