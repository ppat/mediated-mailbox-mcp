//go:build banproof

package core

// This file breaks the pure-core import list on purpose from a pure core whose core path element sits
// at depth three, process/dbconnect/core, and banproof requires the want below. It proves the list's
// file patterns match a pure core at any depth. The database connection's own list and the list for
// non-test code admit the standard library, so only the pure-core list reports it.
import (
	_ "os" // want depguard "import 'os' is not allowed from list 'pure-core'"
)
