// Package report_test is the generator reports the report's tests run. Each requires both halves of
// the range 0 to 9. FIXTURE_VARIANT=narrow draws only the low half, which changes which values the
// draws produce without refusing any.
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

// TestReach draws its cases two ways, one only from the low half and one only from the high half.
// Every case reaches "drawn", and a case from the high half also reaches "high", so "high" is
// reached only by cases that reach another kind first, and only by one of the draws.
func TestReach(t *testing.T) {
	low, high := 5, 9
	if os.Getenv("FIXTURE_VARIANT") == "narrow" {
		low, high = 0, 4
	}
	draws := map[string]func(*rapid.T) int{
		"low half":  func(t *rapid.T) int { return rapid.IntRange(0, 4).Draw(t, "n") },
		"high half": func(t *rapid.T) int { return rapid.IntRange(low, high).Draw(t, "n") },
	}
	property.ReportReach(t, draws, func(n int) []string {
		if n < 5 {
			return []string{"drawn"}
		}
		return []string{"drawn", "high"}
	}, map[string]float64{"drawn": 0.9, "high": 0.2})
}
