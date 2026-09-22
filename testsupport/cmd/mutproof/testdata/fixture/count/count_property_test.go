package count_test

import (
	"testing"

	"pgregory.net/rapid"

	"fixture/count"
)

// TestHoldsForEveryCase numbers the cases rapid generates, so its outcome under the patch depends
// only on how many cases rapid runs.
func TestHoldsForEveryCase(t *testing.T) {
	cases := 0
	rapid.Check(t, func(t *rapid.T) {
		cases++
		if !count.Holds(cases) {
			t.Fatalf("the mechanism failed at case %d", cases)
		}
	})
}
