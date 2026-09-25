package core

import (
	"math"
	"slices"
)

// The latency baseline's windows (ADR-0024).
const (
	latencyWindowMillis = 60_000
	// minWindowSamples is the fewest samples a window needs to have a median.
	minWindowSamples = 20
	// baselineMedians is how many of the latest window medians the baseline is the lowest of.
	baselineMedians = 10
)

// LatencyWindow is one minute of latency samples, in milliseconds, on the caller's monotonic clock.
type LatencyWindow struct {
	// Start is the instant the window opened.
	Start int64
	// Samples are the latencies measured since it opened.
	Samples []int64
}

// Add adds a latency measured at the instant now. When now is a minute or more past the window's
// start, the window has closed, and Add returns a new window opening at now with this sample alone,
// along with the closed window's median and whether it had one. A negative latency is not a
// measurement and is left out. Add appends to Samples as append does.
func (w LatencyWindow) Add(now, latency int64) (next LatencyWindow, median float64, ok bool) {
	if now >= addSaturating(w.Start, latencyWindowMillis) {
		median, ok = Median(w.Samples)
		w = LatencyWindow{Start: now}
	}
	if latency >= 0 {
		w.Samples = append(w.Samples, latency)
	}
	return w, median, ok
}

// Median returns the median of a window's samples, and false when the window holds fewer than 20.
func Median(samples []int64) (float64, bool) {
	if len(samples) < minWindowSamples {
		return 0, false
	}
	sorted := slices.Sorted(slices.Values(samples))
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid]), true
	}
	return (float64(sorted[mid-1]) + float64(sorted[mid])) / 2, true
}

// RememberMedian returns the latest window medians with median added, keeping the last ten.
func RememberMedian(recent []float64, median float64) []float64 {
	kept := append(slices.Clone(recent), median)
	return kept[max(len(kept)-baselineMedians, 0):]
}

// Baseline returns the lowest of the last ten window medians, so a rise in latency becomes the
// baseline only once it has lasted ten windows. A median that is not a finite positive number is
// passed over, and with none left the baseline is zero, which decides nothing.
func Baseline(recent []float64) float64 {
	lowest := math.Inf(1)
	for _, m := range recent[max(len(recent)-baselineMedians, 0):] {
		if m > 0 && m < lowest {
			lowest = m
		}
	}
	if math.IsInf(lowest, 1) {
		return 0
	}
	return lowest
}
