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
// deployable's internal pure core, a library's pure core, a pure core whose core element sits at
// depth three, a composition root writing and linking to them from outside, and two directories
// whose names only start or end with core, which are not pure cores.
func TestGlobals(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp/"
	analysistest.Run(t, analysistest.TestData(), analysis.Globals,
		m+"core/state", m+"mediate/internal/core/plan", m+"ratelimit/core/limits", m+"mediate/wiring",
		m+"corex/lookalike", m+"tools/hardcore/lookalike", m+"tools/extra/core/nested")
}

// The cases under testdata/src stand for a deployable's composition root and the rest of its
// package, its internal code, a shared library with its test file, the test tooling, an operator
// command, the module's root, functions of other packages named like the readers, and a package
// outside this module.
func TestEnvironment(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp"
	analysistest.Run(t, analysistest.TestData(), analysis.Environment,
		m, m+"/backfill", m+"/backfill/internal/run", m+"/process/settings", m+"/testsupport/tool",
		m+"/provider/gmail/cmd/consent", m+"/core/names", "example.com/other")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp/txuser stand for code calling
// generated subsections, one nested a directory down, with its test file, beside a package under db
// whose New builds no Queries and one outside db whose New does.
func TestTxHelper(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysis.TxHelper, "github.com/ppat/mediated-mailbox-mcp/txuser")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp stand for the UI's server with its
// recording mux and a test file, another UI package declaring a type named like the recording mux, the
// UI's composition root, a directory whose name only starts with ui, and the mediator.
func TestRoutes(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp"
	analysistest.Run(t, analysistest.TestData(), analysis.Routes,
		m+"/ui/internal/api", m+"/ui/internal/other", m+"/ui", m+"/uix/lookalike", m+"/mediate/mount")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp stand for a UI package running
// statements through the driver, a generated subsection's handle, an interface embedding it, a
// wrapper and an interface it declares, with lookalikes and a test file, a subsection under db running
// one, and a directory whose name only starts with ui.
func TestRawSQL(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp"
	analysistest.Run(t, analysistest.TestData(), analysis.RawSQL,
		m+"/ui/internal/store", m+"/db/statements", m+"/uix/lookalike")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp stand for a job kind's code in the
// worker with its test file, the worker's entry, its scheduler, and a directory whose name only starts
// with worker.
func TestGoroutines(t *testing.T) {
	const m = "github.com/ppat/mediated-mailbox-mcp"
	analysistest.Run(t, analysistest.TestData(), analysis.Goroutines,
		m+"/worker/internal/job", m+"/worker", m+"/worker/internal/schedule", m+"/workerx/lookalike")
}

// The cases under testdata/src/github.com/ppat/mediated-mailbox-mcp stand for project code importing
// unsafe, with a test file that may, and a package outside this module, which is not checked.
func TestUnsafeImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysis.UnsafeImport,
		"github.com/ppat/mediated-mailbox-mcp/worker/internal/unsafeuser", "example.com/other")
}
