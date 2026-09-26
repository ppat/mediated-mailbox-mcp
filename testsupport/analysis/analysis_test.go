package analysis_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/analysis"
)

// The cases under testdata/src/placement state each finding in a want comment. analysistest fails on
// a want nothing reported and on a finding no want states.
func TestPlacement(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysis.Placement, "placement")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp stand for a pure core, a
// deployable's internal pure core, a library's pure core, a composition root writing and linking to
// them from outside, a directory whose name only starts with core, and a directory named core two
// levels down, which is not a pure core either.
func TestGlobals(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp/"
	analysistest.Run(t, analysistest.TestData(), analysis.Globals,
		m+"core/state", m+"mediate/internal/core/plan", m+"ratelimit/core/limits", m+"mediate/wiring",
		m+"corex/lookalike", m+"tools/extra/core/nested")
}

// The cases under testdata/src stand for a deployable's composition root and the rest of its
// package, its internal code, a shared library with its test file, the test tooling, an operator
// command, the module's root, functions of other packages named like the readers, and a package
// outside this module.
func TestEnvironment(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp"
	analysistest.Run(t, analysistest.TestData(), analysis.Environment,
		m, m+"/backfill", m+"/backfill/internal/run", m+"/settings", m+"/testsupport/tool",
		m+"/provider/gmail/cmd/consent", m+"/core/names", "example.com/other")
}
