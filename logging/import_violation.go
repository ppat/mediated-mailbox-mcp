//go:build banproof

package logging

// This file breaks the logging library's import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'logging'" depguard "list 'non-test-code'"
)
