//go:build banproof

package core

// This file breaks the pure-core import list on purpose from a library's core directory.
import (
	_ "os" // want depguard "import 'os' is not allowed from list 'pure-core'"
)
