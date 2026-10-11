//go:build banproof

package dbmetrics

// This file breaks the database measurements' import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'dbmetrics'" depguard "list 'non-test-code'"
)
