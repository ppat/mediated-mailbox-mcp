//go:build banproof

package schedule

// This file breaks the scheduler's import list on purpose. The import names a third-party package no
// list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'schedule'" depguard "list 'non-test-code'"
)
