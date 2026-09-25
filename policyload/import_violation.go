//go:build banproof

package policyload

// This file breaks the policy loader's import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'policyload'" depguard "list 'non-test-code'"
)
