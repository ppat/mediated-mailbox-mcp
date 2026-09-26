//go:build banproof

package core

// This file breaks the pure-core import list on purpose from a deployable's private core, and sits
// directly in internal/core so a glob that misses files directly in a directory is caught. The list
// keeping HTTP out of every mediator package but the two roots reports it too, though it names no
// package, so a package it was never told about is covered.
import (
	_ "net/http" // want depguard "import 'net/http' is not allowed from list 'pure-core'" depguard "list 'mediate-no-http'"
)
