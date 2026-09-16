//go:build banproof

package sensitivity_test

// This file breaks the pure-core test import list on purpose, and banproof requires every want below.
import (
	_ "os" // want depguard "import 'os' is not allowed from list 'pure-core-tests'"
)
