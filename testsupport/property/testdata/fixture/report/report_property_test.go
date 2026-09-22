// Package report_test is the generator report the report's tests run. It requires both halves of
// the range. FIXTURE_VARIANT=narrow draws only the low half, which changes which values the draw
// produces without refusing any.
package report_test

import (
	"os"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

func TestMix(t *testing.T) {
	high := 9
	if os.Getenv("FIXTURE_VARIANT") == "narrow" {
		high = 4
	}
	property.Report(t, func(t *rapid.T) int { return rapid.IntRange(0, high).Draw(t, "n") }, func(n int) string {
		if n < 5 {
			return "low"
		}
		return "high"
	}, map[string]float64{"low": 0.2, "high": 0.2})
}
