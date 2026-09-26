//go:build banproof

package open

// This file breaks the credential library's import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'credential'" depguard "list 'non-test-code'"
)
