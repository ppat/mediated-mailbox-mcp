//go:build banproof

package process

// This file breaks the process package's import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'process'" depguard "list 'non-test-code'"
)
