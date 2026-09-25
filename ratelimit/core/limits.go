package core

import "math"

// The fractions of the declared ceiling that fix the controller's range and the cap (ADR-0024).
const (
	targetFraction  = 0.50
	hardCapFraction = 0.80
	floorFraction   = 0.05
)

// Limits are the rates a declared ceiling fixes, in the provider's units per second.
type Limits struct {
	// Target is the polite default and the highest rate the controller reaches.
	Target float64
	// HardCap is the most tokens issued inside any one-second window.
	HardCap float64
	// Floor is the lowest rate the controller goes to.
	Floor float64
}

// LimitsFor returns the limits the declared ceiling fixes. A ceiling that is not a finite positive
// number, or is so small that its floor rounds to zero, fixes no budget, and every limit is zero.
// Every rule here computes its limits through this function from the ceiling it is given, and
// none reads a limit from stored state.
func LimitsFor(ceiling float64) Limits {
	floor := ceiling * floorFraction
	if !(floor > 0) || math.IsInf(ceiling, 1) {
		return Limits{}
	}
	return Limits{
		Target:  ceiling * targetFraction,
		HardCap: ceiling * hardCapFraction,
		Floor:   floor,
	}
}

// bounded returns rate brought inside the floor and the target. A rate that is not a number is
// taken as the floor.
func (l Limits) bounded(rate float64) float64 {
	switch {
	case !(rate > l.Floor):
		return l.Floor
	case rate > l.Target:
		return l.Target
	default:
		return rate
	}
}

// addSaturating returns a + b, held at the int64 limits instead of wrapping.
func addSaturating(a, b int64) int64 {
	switch {
	case b > 0 && a > math.MaxInt64-b:
		return math.MaxInt64
	case b < 0 && a < math.MinInt64-b:
		return math.MinInt64
	default:
		return a + b
	}
}
