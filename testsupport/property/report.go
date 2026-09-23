package property

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Report runs draw in a property of its own, classifies every generated case into a kind, and
// fails the test when a kind's share of the cases falls below its stated minimum. It logs the whole
// mix either way.
//
// Call it from its own test, with the draw function a property test uses and the same seed and case
// count, so it counts the cases that test generates. The property it runs holds nothing that can
// fail, the draw and the classification included, because rapid re-runs a failing property many
// times while reducing it and every re-run would be counted. A draw the constructors refuse is its
// own kind, which is how the report shows most of a run being refused.
func Report[A any](t *testing.T, draw func(*rapid.T) A, classify func(A) string, minimums map[string]float64) {
	t.Helper()
	requireSettings(t)
	counts := map[string]int{}
	rapid.Check(t, func(rt *rapid.T) { counts[classify(draw(rt))]++ })
	mix, short := evaluate(counts, minimums)
	t.Logf("generated mix: %s", mix)
	if short != "" {
		t.Errorf("the generator missed its stated mix: %s", short)
	}
}

// evaluate describes the mix and every kind whose share is below its minimum.
func evaluate(counts map[string]int, minimums map[string]float64) (mix, short string) {
	total := 0
	for _, n := range counts {
		total += n
	}
	share := func(kind string) float64 {
		if total == 0 {
			return 0
		}
		return float64(counts[kind]) / float64(total)
	}
	var parts, missed []string
	for _, kind := range slices.Sorted(maps.Keys(counts)) {
		parts = append(parts, fmt.Sprintf("%s %d (%.1f%%)", kind, counts[kind], 100*share(kind)))
	}
	for _, kind := range slices.Sorted(maps.Keys(minimums)) {
		if got := share(kind); got < minimums[kind] {
			missed = append(missed, fmt.Sprintf("%s at %.1f%% of %d cases, below the minimum of %.1f%%", kind, 100*got, total, 100*minimums[kind]))
		}
	}
	return strings.Join(parts, ", "), strings.Join(missed, ", ")
}
