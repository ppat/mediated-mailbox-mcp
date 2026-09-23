//go:build banproof

package analysis_test

// This file breaks the placement analyser's report rule on purpose, and banproof requires the want
// below from go vet.

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

func TestReportInsideAProperty(t *testing.T) {
	draw := func(t *rapid.T) bool { return rapid.Bool().Draw(t, "b") }
	rapid.Check(t, func(rt *rapid.T) {
		property.Report(t, draw, func(bool) string { return "" }, nil) // want placement "property.Report is reachable from inside a property"
	})
}
