//go:build banproof

package runtimemetrics

// This file breaks the runtime measurements' import list on purpose. The import names a third-party
// package no list over shipped code admits.
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'runtimemetrics'" depguard "list 'non-test-code'"
)
