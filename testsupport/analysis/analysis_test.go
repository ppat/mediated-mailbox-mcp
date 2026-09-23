package analysis_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/analysis"
)

// The cases under testdata/src/placement state each finding in a want comment. analysistest fails on
// a want nothing reported and on a finding no want states.
func TestPlacement(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysis.Analyzer, "placement")
}
