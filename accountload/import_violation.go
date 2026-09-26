//go:build banproof

package accountload

// This file breaks the account snapshot library's import list on purpose. The import names a
// third-party package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'accountload'" depguard "list 'non-test-code'"
)
