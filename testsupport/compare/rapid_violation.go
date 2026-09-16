//go:build banproof

package compare

// This file imports rapid into the comparison support on purpose. Every test file imports this
// package, so rapid must stay out of it.
import (
	_ "pgregory.net/rapid" // want depguard "import 'pgregory.net/rapid' is not allowed from list 'testsupport-without-rapid'"
)
