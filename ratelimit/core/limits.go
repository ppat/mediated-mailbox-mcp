package core

import (
	"errors"
	"math"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The fractions of the declared ceiling that fix the controller's range (ADR-0024). The hard cap's
// fraction is mail.HardCapFraction, which the adapters size their calls by.
const (
	// defaultTargetFraction is the target an account has unless its configuration lowers it, and
	// the highest target any account may have.
	defaultTargetFraction = 0.50
	floorFraction         = 0.05
)

// ErrTargetOutOfRange is returned for a target that is not a fraction of the ceiling above the
// floor's and at most the default target's (ADR-0024).
var ErrTargetOutOfRange = errors.New("ratelimit: the target must lie above the floor and at most half the ceiling")

// Limits are the rates a declared ceiling and a target fix, in the provider's units per second. They
// are built only by LimitsFor and LimitsWithTarget, so no caller can raise the target above half the
// ceiling or move the hard cap or the floor. Every rule here decides under the Limits it is given,
// and none reads a limit from stored state. The zero value fixes no budget, and every request under
// it is refused.
type Limits struct {
	target  float64
	hardCap float64
	floor   float64
}

// Target is the polite default and the highest rate the controller reaches, half the ceiling
// unless the account lowered it.
func (l Limits) Target() float64 { return l.target }

// HardCap is the most tokens issued inside any one-second window.
func (l Limits) HardCap() float64 { return l.hardCap }

// Floor is the lowest rate the controller goes to.
func (l Limits) Floor() float64 { return l.floor }

// LimitsFor returns the limits the declared ceiling fixes at the default target. A ceiling that is
// not a finite positive number, or is so small that its floor rounds to zero, fixes no budget, and
// every limit is zero.
func LimitsFor(ceiling float64) Limits {
	return limitsAt(ceiling, defaultTargetFraction)
}

// LimitsWithTarget returns the limits the declared ceiling fixes with the target at the fraction
// target of it. An account's configuration may lower the target to any value above the floor, and
// the hard cap and the floor stay where the ceiling puts them. A target above half the ceiling, at
// the floor or below it, or not a number is refused with ErrTargetOutOfRange. A target at the floor
// would hold the rate there, which is the collapse the alerting rules page on. The rate state stores
// the rate and the collector emits the floor as four-byte floats, so a lowered target that would be
// stored equal to the floor, or below it, is refused too, however far above the floor its fraction
// lies in full precision. A ceiling that fixes no budget gives the zero Limits, as LimitsFor does.
func LimitsWithTarget(ceiling, target float64) (Limits, error) {
	if !(target > floorFraction && target <= defaultTargetFraction) {
		return Limits{}, ErrTargetOutOfRange
	}
	l := limitsAt(ceiling, target)
	if target < defaultTargetFraction && l.floor > 0 && !(float32(l.target) > float32(l.floor)) {
		return Limits{}, ErrTargetOutOfRange
	}
	return l, nil
}

// limitsAt returns the limits the declared ceiling fixes with the target at a fraction already
// checked, or the zero Limits for a ceiling that fixes no budget.
func limitsAt(ceiling, target float64) Limits {
	floor := ceiling * floorFraction
	if !(floor > 0) || math.IsInf(ceiling, 1) {
		return Limits{}
	}
	return Limits{
		target:  ceiling * target,
		hardCap: ceiling * mail.HardCapFraction,
		floor:   floor,
	}
}

// bounded returns rate brought inside the floor and the target. A rate that is not a number is
// taken as the floor.
func (l Limits) bounded(rate float64) float64 {
	switch {
	case !(rate > l.floor):
		return l.floor
	case rate > l.target:
		return l.target
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
