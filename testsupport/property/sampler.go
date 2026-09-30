package property

import (
	"slices"

	"pgregory.net/rapid"
)

// Sampler draws operation names under a set of weights drawn fresh for each generated sequence, the
// operation sampler of ADR-0069. rapid's own draw picks an operation by its position among the sorted
// names, so renaming an operation changes how often it runs, and fixed weights tuned for one target
// can hide a failure of another. A fresh set per sequence reaches mixes no fixed set does.
//
// The weights are drawn from rapid's own random stream, so a sequence rapid replays while reducing it
// is drawn under the same weights (row 12 of ADR-0069's ordinary-path table). They exist only inside
// the draw. A stored failing case holds the operations drawn and never the weights, and a failure
// report describes the operations, because reduction flattens the weights while keeping the sequence
// (rows 13 and 14).
type Sampler struct {
	names   []string
	weights []float64
	total   float64
}

// NewSampler draws a weight between 0 and 1 for each of names, sorted, from t. Reduction settles each
// draw on the one number that zero bits give once mixed, so a reduced sequence weighs every name the
// same. Weights summing to zero, which the mixing makes all but impossible, weigh every name the same
// too.
func NewSampler(t *rapid.T, names []string) Sampler {
	sorted := slices.Sorted(slices.Values(names))
	weights := make([]float64, 0, len(sorted))
	for _, name := range sorted {
		weights = append(weights, uniform(t, "weight of "+name))
	}
	return weighted(sorted, weights)
}

// weighted returns the Sampler drawing names under weights, every name weighing the same when every
// weight is zero.
func weighted(names []string, weights []float64) Sampler {
	s := Sampler{names: names, weights: weights}
	for _, w := range weights {
		s.total += w
	}
	if s.total == 0 {
		s.weights = make([]float64, len(names))
		for i := range s.weights {
			s.weights[i] = 1
		}
		s.total = float64(len(names))
	}
	return s
}

// Draw draws one name, each with the chance its weight's share of the total gives it.
func (s Sampler) Draw(t *rapid.T, label string) string {
	point := uniform(t, label) * s.total
	for i, w := range s.weights {
		if point < w {
			return s.names[i]
		}
		point -= w
	}
	// The point was drawn at the total itself, which lies past every weight.
	for i := len(s.weights) - 1; i >= 0; i-- {
		if s.weights[i] > 0 {
			return s.names[i]
		}
	}
	return s.names[len(s.names)-1]
}

// uniform draws a number in [0, 1) from t, spread evenly. rapid draws numbers that lean toward zero,
// small values and the ends of a range, which is what reduction wants and would skew every share, so
// the drawn bits are offset and mixed as SplitMix64 mixes its state. Reduction still settles each draw
// on one fixed number, the one zero bits give.
func uniform(t *rapid.T, label string) float64 {
	x := rapid.Uint64().Draw(t, label) + 0x9e3779b97f4a7c15
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}
