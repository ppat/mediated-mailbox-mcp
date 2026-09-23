//go:build banproof

package classify

// This file breaks the import lists for pure cores and for non-test code on purpose, and banproof
// requires the wants below. Neither package is pure, so the classifier's callers pass their
// functions in. The first caller admits the packages it passes to the list for non-test code and
// drops that list's wants here.
import (
	_ "golang.org/x/net/idna"         // want depguard "list 'pure-core'" depguard "list 'non-test-code'"
	_ "golang.org/x/net/publicsuffix" // want depguard "list 'pure-core'" depguard "list 'non-test-code'"
)
