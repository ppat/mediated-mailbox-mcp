//go:build banproof

package analysis_test

// This file breaks the placement analyser's failing-case store rule on purpose, and banproof
// requires the want below from go vet.

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

func TestCheckInsideAProperty(t *testing.T) {
	draw := func(t *rapid.T) bool { return rapid.Bool().Draw(t, "b") }
	rapid.Check(t, func(rt *rapid.T) {
		property.Check(t, draw, func(rapid.TB, bool) {}) // want placement "property.Check is reachable from inside a property"
	})
}
