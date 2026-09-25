package core_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// samples returns n latencies of the given milliseconds.
func samples(n int, ms int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = ms
	}
	return out
}

func TestMedian(t *testing.T) {
	ascending := make([]int64, 21)
	for i := range ascending {
		ascending[i] = int64(100 - i)
	}
	cases := []struct {
		name    string
		samples []int64
		want    float64
		wantOK  bool
	}{
		{"none", nil, 0, false},
		{"19 samples are too few", samples(19, 50), 0, false},
		{"20 samples are enough", samples(20, 50), 50, true},
		{"an odd count takes the middle sample, whatever the order", ascending, 90, true},
		{"an even count takes the mean of the two middle samples", append(samples(10, 40), samples(10, 60)...), 50, true},
		{"the largest latencies do not overflow", samples(20, math.MaxInt64), math.MaxInt64, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := core.Median(c.samples)
			if diff := cmp.Diff([]any{c.want, c.wantOK}, []any{got, ok}, compare.Options); diff != "" {
				t.Errorf("Median (-want +got):\n%s", diff)
			}
		})
	}
}

// The median does not reorder the caller's samples.
func TestMedianLeavesTheSamplesInPlace(t *testing.T) {
	in := []int64{9, 1, 8, 2, 7, 3, 6, 4, 5, 0, 19, 11, 18, 12, 17, 13, 16, 14, 15, 10}
	kept := append([]int64(nil), in...)
	core.Median(in)
	if diff := cmp.Diff(kept, in, compare.Options); diff != "" {
		t.Errorf("Median changed its input (-want +got):\n%s", diff)
	}
}

func TestLatencyWindowAdd(t *testing.T) {
	type result struct {
		Next   core.LatencyWindow
		Median float64
		OK     bool
	}
	cases := []struct {
		name    string
		window  core.LatencyWindow
		now     int64
		latency int64
		want    result
	}{
		{
			"a sample inside the minute joins the window",
			core.LatencyWindow{Start: 1000, Samples: []int64{5}},
			60_999, 7,
			result{Next: core.LatencyWindow{Start: 1000, Samples: []int64{5, 7}}},
		},
		{
			"a sample a minute after the start closes the window with its median",
			core.LatencyWindow{Start: 1000, Samples: samples(20, 30)},
			61_000, 7,
			result{Next: core.LatencyWindow{Start: 61_000, Samples: []int64{7}}, Median: 30, OK: true},
		},
		{
			"a window closing with too few samples has no median",
			core.LatencyWindow{Start: 1000, Samples: samples(19, 30)},
			90_000, 7,
			result{Next: core.LatencyWindow{Start: 90_000, Samples: []int64{7}}},
		},
		{
			"a negative latency is left out",
			core.LatencyWindow{Start: 1000, Samples: []int64{5}},
			2000, -3,
			result{Next: core.LatencyWindow{Start: 1000, Samples: []int64{5}}},
		},
		{
			"a zero latency is a sample",
			core.LatencyWindow{Start: 1000, Samples: []int64{5}},
			2000, 0,
			result{Next: core.LatencyWindow{Start: 1000, Samples: []int64{5, 0}}},
		},
		{
			"a window opened near the clock's limit does not wrap and close early",
			core.LatencyWindow{Start: math.MaxInt64 - 10, Samples: samples(20, 30)},
			math.MaxInt64 - 5, 7,
			result{Next: core.LatencyWindow{Start: math.MaxInt64 - 10, Samples: append(samples(20, 30), 7)}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, median, ok := c.window.Add(c.now, c.latency)
			if diff := cmp.Diff(c.want, result{next, median, ok}, compare.Options); diff != "" {
				t.Errorf("Add (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRememberMedian(t *testing.T) {
	cases := []struct {
		name   string
		recent []float64
		median float64
		want   []float64
	}{
		{"the first", nil, 40, []float64{40}},
		{"added last", []float64{40, 50}, 30, []float64{40, 50, 30}},
		{"the eleventh drops the oldest", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 11, []float64{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		{"a longer stored list keeps its last nine", []float64{0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 10, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.RememberMedian(c.recent, c.median), compare.Options); diff != "" {
				t.Errorf("RememberMedian (-want +got):\n%s", diff)
			}
		})
	}
}

// Remembering a median does not write into the caller's list.
func TestRememberMedianLeavesTheListInPlace(t *testing.T) {
	backing := []float64{1, 2, 3, 0, 0}
	core.RememberMedian(backing[:3], 4)
	if diff := cmp.Diff([]float64{1, 2, 3, 0, 0}, backing, compare.Options); diff != "" {
		t.Errorf("RememberMedian wrote into its input (-want +got):\n%s", diff)
	}
}

func TestBaseline(t *testing.T) {
	cases := []struct {
		name   string
		recent []float64
		want   float64
	}{
		{"none", nil, 0},
		{"the lowest", []float64{50, 30, 40}, 30},
		{"only the last ten count", []float64{5, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140}, 50},
		{"a sustained rise becomes the baseline after ten windows", []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100}, 100},
		{"a zero median is passed over", []float64{0, 40}, 40},
		{"a negative median is passed over", []float64{-5, 40}, 40},
		{"a median that is not a number is passed over", []float64{math.NaN(), 40}, 40},
		{"an infinite median is passed over", []float64{math.Inf(1), 40}, 40},
		{"nothing usable is no baseline", []float64{0, math.NaN(), math.Inf(1)}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.Baseline(c.recent), compare.Options); diff != "" {
				t.Errorf("Baseline (-want +got):\n%s", diff)
			}
		})
	}
}

// The baseline rule end to end. Ten one-minute windows at 50 ms set the baseline, a window at 101 ms
// cuts the rate, and a rise that lasts ten windows becomes the baseline and stops cutting.
func TestALastingRiseBecomesTheBaseline(t *testing.T) {
	var recent []float64
	s := core.State{Rate: 100}
	closeWindow := func(median float64) {
		s = core.LatencyMeasured(s, declared, median, core.Baseline(recent))
		recent = core.RememberMedian(recent, median)
	}
	for range 10 {
		closeWindow(50)
	}
	if s.Rate != 100 {
		t.Fatalf("steady latency moved the rate to %v", s.Rate)
	}
	closeWindow(101)
	if s.Rate != 90 {
		t.Fatalf("a median above twice the baseline gave rate %v, want 90", s.Rate)
	}
	for range 9 {
		closeWindow(101)
	}
	if got := core.Baseline(recent); got != 101 {
		t.Fatalf("after ten windows at 101 the baseline is %v, want 101", got)
	}
	before := s.Rate
	closeWindow(150)
	if s.Rate != before {
		t.Errorf("a median below twice the new baseline moved the rate from %v to %v", before, s.Rate)
	}
}
