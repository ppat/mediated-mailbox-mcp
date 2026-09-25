package core

import (
	"math"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The controller's steps and factors (ADR-0024).
const (
	// stepFraction is the additive step as a fraction of the target, before it is scaled by the
	// call's cost over the current rate.
	stepFraction = 0.02
	// The multiplicative decreases, one per signal.
	throttleFactor    = 0.5
	serverErrorFactor = 0.8
	latencyFactor     = 0.9
	// latencyTrigger is how many times the baseline a median must exceed to cut the rate.
	latencyTrigger = 2
	// The full-jitter bound starts at the base and doubles per throttle up to the most.
	backoffBaseMillis = 1000
	backoffMostMillis = 32000
)

// State is the controller's state for one account.
type State struct {
	// Rate is the current rate, in the provider's units per second.
	Rate float64
	// BackoffUntil is the instant before which nothing is issued.
	BackoffUntil int64
	// Throttles counts the throttles since the last success.
	Throttles int
}

// Succeeded returns the state after a call of the given cost succeeded. The rate grows by 2% of the
// target times the cost divided by the current rate, up to the target, so a second whose successes
// spend the whole rate grows it by 2% of the target. A success ends a run of throttles.
func Succeeded(s State, l Limits, cost mail.OpCost) State {
	rate := l.bounded(s.Rate)
	if w := cost.Weight; w > 0 && !math.IsInf(w, 1) && rate > 0 {
		rate = min(rate+stepFraction*l.target*w/rate, l.target)
	}
	return State{Rate: rate, BackoffUntil: s.BackoffUntil}
}

// Throttled returns the state after the provider throttled a call at the instant now. The rate
// halves, down to the floor. Nothing is issued until the retry-after the signal carries has passed,
// or without one until a full-jitter backoff drawn with draw has passed. A throttle arriving during
// a backoff never ends it sooner, so a retry-after is honored in full.
func Throttled(s State, l Limits, signal mail.ThrottleSignal, now int64, draw float64) State {
	throttles := max(s.Throttles, 0)
	wait := jitter(throttles, draw)
	if signal.HasRetryAfter && signal.RetryAfterMillis > 0 {
		wait = signal.RetryAfterMillis
	}
	if throttles < math.MaxInt {
		throttles++
	}
	return State{
		Rate:         max(l.bounded(s.Rate)*throttleFactor, l.floor),
		BackoffUntil: max(s.BackoffUntil, addSaturating(now, wait)),
		Throttles:    throttles,
	}
}

// ServerErrored returns the state after the provider failed a call with a server error. The rate
// falls to 80%, down to the floor.
func ServerErrored(s State, l Limits) State {
	return State{
		Rate:         max(l.bounded(s.Rate)*serverErrorFactor, l.floor),
		BackoffUntil: s.BackoffUntil,
		Throttles:    s.Throttles,
	}
}

// LatencyMeasured returns the state after a latency window closed with the given median. The rate
// falls to 90%, down to the floor, when the median is more than twice the baseline. A baseline that
// is not a finite positive number, which includes no baseline yet, decides nothing.
func LatencyMeasured(s State, l Limits, median, baseline float64) State {
	rate := l.bounded(s.Rate)
	if baseline > 0 && !math.IsInf(baseline, 1) && !math.IsInf(median, 1) && median > latencyTrigger*baseline {
		rate = max(rate*latencyFactor, l.floor)
	}
	return State{Rate: rate, BackoffUntil: s.BackoffUntil, Throttles: s.Throttles}
}

// jitter returns a full-jitter backoff in milliseconds, the draw times one second doubled once per
// earlier throttle and held at 32 seconds.
func jitter(throttles int, draw float64) int64 {
	bound := int64(backoffBaseMillis)
	for i := 0; i < throttles && bound < backoffMostMillis; i++ {
		bound *= 2
	}
	bound = min(bound, backoffMostMillis)
	if !(draw >= 0 && draw < 1) {
		return bound
	}
	return int64(draw * float64(bound))
}
