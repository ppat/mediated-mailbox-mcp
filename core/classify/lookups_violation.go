//go:build banproof

package classify

// This file breaks the import list for pure cores on purpose, and banproof requires the wants below.
// Neither package is pure, so the classifier's callers pass their functions in. The list for
// non-test code admits both, for the mediator's composition root, which passes them.
import (
	_ "golang.org/x/net/idna"         // want depguard "list 'pure-core'"
	_ "golang.org/x/net/publicsuffix" // want depguard "list 'pure-core'"
)
