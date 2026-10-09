//go:build banproof

package core_test

// This file imports a logging package into a pure core's test file on purpose, and banproof requires
// the want below.
import (
	_ "log/slog" // want depguard "import 'log/slog' is not allowed from list 'pure-core-tests'"
)
