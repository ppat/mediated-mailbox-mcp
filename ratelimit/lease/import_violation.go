//go:build banproof

package lease

// This file breaks the rate limiter's import list on purpose. The import names a third-party
// package, because which way the rate limiter and the provider adapters depend on each other is
// open, and an import of an adapter here could become an import cycle.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'ratelimit'" depguard "list 'non-test-code'"
)
