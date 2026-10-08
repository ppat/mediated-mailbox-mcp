//go:build banproof

package dbconnect

// This file breaks the database connection's import list on purpose. The import names a
// third-party package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'dbconnect'" depguard "list 'non-test-code'"
)
