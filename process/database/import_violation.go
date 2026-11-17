//go:build banproof

package database

// This file breaks the database package's import list on purpose. The import names a
// third-party package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'database'" depguard "list 'non-test-code'"
)
