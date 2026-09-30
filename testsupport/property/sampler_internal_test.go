package property

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// counts draws n names from s, one example each from seeds 0 to n-1, and counts them.
func counts(s Sampler, n int) map[string]int {
	draw := rapid.Custom(func(t *rapid.T) string { return s.Draw(t, "operation") })
	out := map[string]int{}
	for seed := range n {
		out[draw.Example(seed)]++
	}
	return out
}

// A name is drawn with its weight's share of the total, and a name weighing nothing is never drawn.
func TestTheSamplerDrawsByWeight(t *testing.T) {
	s := weighted([]string{"a", "b", "c"}, []float64{0, 1, 3})
	got := counts(s, 4000)
	if got["a"] != 0 {
		t.Errorf("a name weighing nothing was drawn %d times", got["a"])
	}
	if share := float64(got["c"]) / 4000; share < 0.7 || share > 0.8 {
		t.Errorf("a name weighing three quarters of the total was drawn in %.3f of draws, want about 0.75 (%v)", share, got)
	}
}

// Every name weighs the same when every weight was drawn as zero, as reduction tends to leave them, so
// a reduced sequence still draws each operation.
func TestAllZeroWeightsWeighTheSame(t *testing.T) {
	got := counts(weighted([]string{"a", "b"}, []float64{0, 0}), 2000)
	if got["a"] < 800 || got["b"] < 800 {
		t.Errorf("with every weight zero the draws were %v, want each name about half", got)
	}
}

// The weights come from rapid's own random stream, one set per generated sequence. The same stream
// draws the same weights, so a sequence rapid replays while reducing it is drawn under them, and
// different streams draw different sets. The names are sorted, whatever order they were given in.
func TestTheWeightsComeFromRapidsStream(t *testing.T) {
	sampled := rapid.Custom(func(t *rapid.T) Sampler { return NewSampler(t, []string{"crash", "page", "arrive"}) })
	first, again := sampled.Example(7), sampled.Example(7)
	if diff := cmp.Diff(first.weights, again.weights, compare.Options); diff != "" {
		t.Errorf("one stream drew two sets of weights (-first +again):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"arrive", "crash", "page"}, first.names, compare.Options); diff != "" {
		t.Errorf("names (-want +got):\n%s", diff)
	}
	distinct := map[[3]float64]bool{}
	for seed := range 20 {
		w := sampled.Example(seed).weights
		distinct[[3]float64{w[0], w[1], w[2]}] = true
	}
	if len(distinct) < 10 {
		t.Errorf("twenty streams drew %d distinct sets of weights, want a fresh set for nearly every one", len(distinct))
	}
}
