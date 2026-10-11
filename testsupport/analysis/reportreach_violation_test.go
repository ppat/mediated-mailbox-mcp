//go:build banproof

package analysis_test

// This file breaks the placement analyser's report rule on purpose, for the report over several
// draws, and banproof requires the want below from go vet.

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

func TestReportReachInsideAProperty(t *testing.T) {
	draws := map[string]func(*rapid.T) bool{"one": func(t *rapid.T) bool { return rapid.Bool().Draw(t, "b") }}
	rapid.Check(t, func(rt *rapid.T) {
		property.ReportReach(t, draws, func(bool) []string { return nil }, nil) // want vetcheck "property.ReportReach is reachable from inside a property"
	})
}
