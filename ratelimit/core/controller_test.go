package core_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// declared is the ceiling the tests declare, 250 units a second. Its target is 125, its hard cap 200
// and its floor 12.5.
const declared = 250.0

func cost(weight float64) mail.OpCost { return mail.OpCost{Weight: weight, OpsCount: 1} }

func TestLimitsFor(t *testing.T) {
	cases := []struct {
		name    string
		ceiling float64
		want    core.Limits
	}{
		{"declared", declared, core.Limits{Target: 125, HardCap: 200, Floor: 12.5}},
		{"one unit", 1, core.Limits{Target: 0.5, HardCap: 0.8, Floor: 0.05}},
		{"the largest float", math.MaxFloat64, core.Limits{Target: math.MaxFloat64 * 0.5, HardCap: math.MaxFloat64 * 0.8, Floor: math.MaxFloat64 * 0.05}},
		{"zero", 0, core.Limits{}},
		{"negative", -250, core.Limits{}},
		{"not a number", math.NaN(), core.Limits{}},
		{"positive infinity", math.Inf(1), core.Limits{}},
		{"negative infinity", math.Inf(-1), core.Limits{}},
		{"so small the floor rounds to zero", math.SmallestNonzeroFloat64, core.Limits{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.LimitsFor(c.ceiling), compare.Options); diff != "" {
				t.Errorf("LimitsFor(%v) (-want +got):\n%s", c.ceiling, diff)
			}
		})
	}
}

func TestSucceeded(t *testing.T) {
	cases := []struct {
		name string
		in   core.State
		cost float64
		want core.State
	}{
		// 2% of 125 is 2.5, times a cost of 5, over a rate of 12.5.
		{"from the floor", core.State{Rate: 12.5}, 5, core.State{Rate: 13.5}},
		// The same cost over twice the rate grows it half as much.
		{"from twice the floor", core.State{Rate: 25}, 5, core.State{Rate: 25.5}},
		{"a cost of the whole rate grows it by 2% of the target", core.State{Rate: 100}, 100, core.State{Rate: 102.5}},
		{"held at the target", core.State{Rate: 124}, 100, core.State{Rate: 125}},
		{"at the target", core.State{Rate: 125}, 1, core.State{Rate: 125}},
		{"a success ends a run of throttles and keeps the backoff", core.State{Rate: 50, BackoffUntil: 9000, Throttles: 4}, 1, core.State{Rate: 50.05, BackoffUntil: 9000}},
		{"a zero cost grows nothing", core.State{Rate: 50}, 0, core.State{Rate: 50}},
		{"a negative cost grows nothing", core.State{Rate: 50}, -5, core.State{Rate: 50}},
		{"a cost that is not a number grows nothing", core.State{Rate: 50}, math.NaN(), core.State{Rate: 50}},
		{"an infinite cost grows nothing", core.State{Rate: 50}, math.Inf(1), core.State{Rate: 50}},
		{"a huge cost reaches only the target", core.State{Rate: 50}, math.MaxFloat64, core.State{Rate: 125}},
		{"a stored rate above the target is brought down", core.State{Rate: 1000}, 1, core.State{Rate: 125}},
		{"a stored rate below the floor is brought up", core.State{Rate: 1}, 0, core.State{Rate: 12.5}},
		{"a stored rate that is not a number is taken as the floor", core.State{Rate: math.NaN()}, 0, core.State{Rate: 12.5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.Succeeded(c.in, declared, cost(c.cost)), compare.Options); diff != "" {
				t.Errorf("Succeeded (-want +got):\n%s", diff)
			}
		})
	}
}

// While each second's successes spend the whole rate, the rate climbs from the floor to the target
// in 45 seconds whatever the rate, as ADR-0024 states. One success a second costing the whole rate
// is such a second.
func TestTheRateClimbsFromTheFloorToTheTargetIn45Seconds(t *testing.T) {
	s := core.State{Rate: 12.5}
	seconds := 0
	for s.Rate < 125 && seconds < 1000 {
		s = core.Succeeded(s, declared, cost(s.Rate))
		seconds++
	}
	if seconds != 45 {
		t.Errorf("the rate reached %v after %d seconds, want 125 after 45", s.Rate, seconds)
	}
}

// Ten successes a second, each costing a tenth of the rate at the start of the second, also spend
// the whole rate. The climb takes longer than 45 seconds only because the rate grows within each
// second, so each later success is a smaller share of it.
func TestTheClimbHoldsForManySmallSuccesses(t *testing.T) {
	s := core.State{Rate: 12.5}
	seconds := 0
	for s.Rate < 125 && seconds < 1000 {
		each := s.Rate / 10
		for range 10 {
			s = core.Succeeded(s, declared, cost(each))
		}
		seconds++
	}
	if seconds < 45 || seconds > 47 {
		t.Errorf("the rate reached %v after %d seconds, want 125 after 45 to 47", s.Rate, seconds)
	}
}

func TestServerErrored(t *testing.T) {
	cases := []struct {
		name string
		in   core.State
		want core.State
	}{
		{"falls to 80%", core.State{Rate: 100, BackoffUntil: 7, Throttles: 2}, core.State{Rate: 80, BackoffUntil: 7, Throttles: 2}},
		{"held at the floor", core.State{Rate: 13}, core.State{Rate: 12.5}},
		{"a stored rate above the target falls from the target", core.State{Rate: 1e9}, core.State{Rate: 100}},
		{"a stored rate that is not a number is taken as the floor", core.State{Rate: math.NaN()}, core.State{Rate: 12.5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.ServerErrored(c.in, declared), compare.Options); diff != "" {
				t.Errorf("ServerErrored (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLatencyMeasured(t *testing.T) {
	cases := []struct {
		name             string
		median, baseline float64
		want             float64
	}{
		{"more than twice the baseline cuts to 90%", 201, 100, 90},
		{"exactly twice the baseline does not", 200, 100, 100},
		{"below twice the baseline does not", 150, 100, 100},
		{"no baseline decides nothing", 1e6, 0, 100},
		{"a negative baseline decides nothing", 1e6, -1, 100},
		{"a baseline that is not a number decides nothing", 1e6, math.NaN(), 100},
		{"an infinite baseline decides nothing", 1e6, math.Inf(1), 100},
		{"a median that is not a number decides nothing", math.NaN(), 100, 100},
		{"an infinite median decides nothing", math.Inf(1), 100, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := core.State{Rate: 100, BackoffUntil: 5, Throttles: 1}
			want := core.State{Rate: c.want, BackoffUntil: 5, Throttles: 1}
			if diff := cmp.Diff(want, core.LatencyMeasured(in, declared, c.median, c.baseline), compare.Options); diff != "" {
				t.Errorf("LatencyMeasured (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("held at the floor", func(t *testing.T) {
		got := core.LatencyMeasured(core.State{Rate: 13}, declared, 300, 100)
		if diff := cmp.Diff(core.State{Rate: 12.5}, got, compare.Options); diff != "" {
			t.Errorf("LatencyMeasured (-want +got):\n%s", diff)
		}
	})
}

func TestThrottled(t *testing.T) {
	const now = 1_000_000
	retryAfter := func(ms int64) mail.ThrottleSignal {
		return mail.ThrottleSignal{RetryAfterMillis: ms, HasRetryAfter: true, Scope: mail.ScopePerUser}
	}
	none := mail.ThrottleSignal{}
	cases := []struct {
		name   string
		in     core.State
		signal mail.ThrottleSignal
		now    int64
		draw   float64
		want   core.State
	}{
		{"halves the rate and honors the retry-after", core.State{Rate: 100}, retryAfter(3000), now, 0.5, core.State{Rate: 50, BackoffUntil: now + 3000, Throttles: 1}},
		{"a retry-after of an hour is honored as given", core.State{Rate: 100}, retryAfter(3_600_000), now, 0.5, core.State{Rate: 50, BackoffUntil: now + 3_600_000, Throttles: 1}},
		{"a retry-after of a project throttle is honored the same", core.State{Rate: 100}, mail.ThrottleSignal{RetryAfterMillis: 3000, HasRetryAfter: true, Scope: mail.ScopePerProject}, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 3000, Throttles: 1}},
		{"held at the floor", core.State{Rate: 20}, retryAfter(3000), now, 0.5, core.State{Rate: 12.5, BackoffUntil: now + 3000, Throttles: 1}},
		{"the first throttle draws within one second", core.State{Rate: 100}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 1}},
		{"the bound doubles per earlier throttle", core.State{Rate: 100, Throttles: 3}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 4000, Throttles: 4}},
		{"the bound stops doubling at 32 seconds", core.State{Rate: 100, Throttles: 5}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 16000, Throttles: 6}},
		{"the bound holds at 32 seconds past it", core.State{Rate: 100, Throttles: 10}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 16000, Throttles: 11}},
		{"the bound holds at 32 seconds at the largest count", core.State{Rate: 100, Throttles: math.MaxInt}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 16000, Throttles: math.MaxInt}},
		{"a draw of zero waits nothing", core.State{Rate: 100, Throttles: 2}, none, now, 0, core.State{Rate: 50, BackoffUntil: now, Throttles: 3}},
		{"a draw just below one waits almost the bound", core.State{Rate: 100, Throttles: 2}, none, now, 0.9999, core.State{Rate: 50, BackoffUntil: now + 3999, Throttles: 3}},
		{"a negative count starts over", core.State{Rate: 100, Throttles: -7}, none, now, 0.5, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 1}},
		{"a zero retry-after counts as absent", core.State{Rate: 100, Throttles: 1}, retryAfter(0), now, 0.25, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 2}},
		{"a retry-after in the past counts as absent", core.State{Rate: 100, Throttles: 1}, retryAfter(-5000), now, 0.25, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 2}},
		{"the smallest retry-after counts as absent", core.State{Rate: 100, Throttles: 1}, retryAfter(math.MinInt64), now, 0.25, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 2}},
		{"a retry-after not marked present counts as absent", core.State{Rate: 100, Throttles: 1}, mail.ThrottleSignal{RetryAfterMillis: 3000}, now, 0.25, core.State{Rate: 50, BackoffUntil: now + 500, Throttles: 2}},
		{"the largest retry-after saturates instead of wrapping", core.State{Rate: 100}, retryAfter(math.MaxInt64), now, 0.5, core.State{Rate: 50, BackoffUntil: math.MaxInt64, Throttles: 1}},
		{"a retry-after near the clock's limit saturates", core.State{Rate: 100}, retryAfter(1000), math.MaxInt64 - 10, 0.5, core.State{Rate: 50, BackoffUntil: math.MaxInt64, Throttles: 1}},
		{"a draw of one waits the whole bound", core.State{Rate: 100, Throttles: 2}, none, now, 1, core.State{Rate: 50, BackoffUntil: now + 4000, Throttles: 3}},
		{"a draw above one waits the whole bound", core.State{Rate: 100, Throttles: 2}, none, now, 7, core.State{Rate: 50, BackoffUntil: now + 4000, Throttles: 3}},
		{"a negative draw waits the whole bound", core.State{Rate: 100, Throttles: 2}, none, now, -0.5, core.State{Rate: 50, BackoffUntil: now + 4000, Throttles: 3}},
		{"a draw that is not a number waits the whole bound", core.State{Rate: 100, Throttles: 2}, none, now, math.NaN(), core.State{Rate: 50, BackoffUntil: now + 4000, Throttles: 3}},
		{"a later throttle extends a backoff from its own instant", core.State{Rate: 100, BackoffUntil: now + 500}, retryAfter(1000), now, 0.5, core.State{Rate: 50, BackoffUntil: now + 1000, Throttles: 1}},
		{"a jitter throttle during a one-hour retry-after keeps the hour", core.State{Rate: 100, BackoffUntil: now + 3_600_000, Throttles: 1}, none, now + 1000, 0.5, core.State{Rate: 50, BackoffUntil: now + 3_600_000, Throttles: 2}},
		{"a stored rate above the target halves from the target", core.State{Rate: 1e9}, retryAfter(1000), now, 0.5, core.State{Rate: 62.5, BackoffUntil: now + 1000, Throttles: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.Throttled(c.in, declared, c.signal, c.now, c.draw), compare.Options); diff != "" {
				t.Errorf("Throttled (-want +got):\n%s", diff)
			}
		})
	}
}

// The controller under a ceiling that fixes no budget holds every rate at zero.
func TestNoBudgetHoldsTheRateAtZero(t *testing.T) {
	for _, ceiling := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		s := core.State{Rate: 100}
		s = core.Succeeded(s, ceiling, cost(5))
		if s.Rate != 0 {
			t.Errorf("Succeeded under a ceiling of %v gave rate %v, want 0", ceiling, s.Rate)
		}
		s = core.ServerErrored(core.State{Rate: 100}, ceiling)
		if s.Rate != 0 {
			t.Errorf("ServerErrored under a ceiling of %v gave rate %v, want 0", ceiling, s.Rate)
		}
	}
}
