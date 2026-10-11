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
	report(t, map[string]func(*rapid.T) A{"": draw}, func(a A) []string { return []string{classify(a)} }, minimums)
}

// ReportReach is Report for a test whose cases are drawn several ways in one run and can each reach
// several kinds, as a crash test's operation sequences are (testsupport/crash). It runs each draw in
// a property of its own, counts a case for every kind reached returns for it, and fails the test when
// the share of all the draws' cases reaching a kind falls below the kind's stated minimum, since a
// kind any draw reaches is one the run exercises. It fails only when the draws together miss a kind,
// so one draw alone no longer reaching a kind fails nothing. It logs each draw's mix and the mix of
// every draw together, which a verbose run prints (ADR-0069).
//
// Call it as Report is called, with the draws the test runs and the same seed and case count.
func ReportReach[A any](t *testing.T, draws map[string]func(*rapid.T) A, reached func(A) []string, minimums map[string]float64) {
	t.Helper()
	report(t, draws, reached, minimums)
}

// report runs each draw in a property of its own, in the order of the draws' names, counts each case
// once for every distinct kind reached returns for it, and evaluates the counts over every draw's
// cases together. A draw named "" is the single draw of Report, whose mix is the whole.
func report[A any](t *testing.T, draws map[string]func(*rapid.T) A, reached func(A) []string, minimums map[string]float64) {
	t.Helper()
	requireSettings(t)
	all, total := map[string]int{}, 0
	for _, name := range slices.Sorted(maps.Keys(draws)) {
		counts, cases := map[string]int{}, 0
		rapid.Check(t, func(rt *rapid.T) {
			cases++
			seen := map[string]bool{}
			for _, kind := range reached(draws[name](rt)) {
				if !seen[kind] {
					seen[kind] = true
					counts[kind]++
				}
			}
		})
		for kind, n := range counts {
			all[kind] += n
		}
		total += cases
		if len(draws) > 1 {
			mix, _ := evaluate(counts, cases, nil)
			t.Logf("generated mix of %s: %s", name, mix)
		}
	}
	mix, short := evaluate(all, total, minimums)
	if len(draws) > 1 {
		t.Logf("generated mix of every draw: %s", mix)
	} else {
		t.Logf("generated mix: %s", mix)
	}
	if short != "" {
		t.Errorf("the generator missed its stated mix: %s", short)
	}
}

// evaluate describes the mix of total cases and every kind whose share of them is below its minimum.
func evaluate(counts map[string]int, total int, minimums map[string]float64) (mix, short string) {
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
