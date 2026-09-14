//go:build banproof

package tx_test

// This file imports golang.org/x/tools into an ordinary test file outside the shared test tooling on
// purpose.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'db'" depguard "list 'ordinary-tests'"
)
