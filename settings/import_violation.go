//go:build banproof

package settings

// This file breaks the configuration library's import list on purpose. The import names a
// third-party package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'settings'" depguard "list 'non-test-code'"
)
