//go:build banproof

package seal

// This file breaks the sealing half's import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'credential-seal'" depguard "list 'non-test-code'"
)
