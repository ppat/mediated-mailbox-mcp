package core_test

import (
	"math"
	"slices"
	"strconv"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// The operations a run is made of.
const (
	opRequest = iota
	opSuccess
	opThrottle
	opServerError
	opLatency
	opCorruptRate
	opStoreLease
	opPrune
	opCorruptInstant
	opCount
)

// number is a float64 the failing-case store can keep whatever its value. The store writes JSON,
// which has no spelling for not-a-number or the infinities, so number writes those three as the
// strings strconv reads back.
type number float64

func (n number) MarshalJSON() ([]byte, error) {
	f := float64(n)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte(strconv.Quote(strconv.FormatFloat(f, 'g', -1, 64))), nil
	}
	return []byte(strconv.FormatFloat(f, 'g', -1, 64)), nil
}

func (n *number) UnmarshalJSON(src []byte) error {
	text := string(src)
	if unquoted, err := strconv.Unquote(text); err == nil {
		text = unquoted
	}
	f, err := strconv.ParseFloat(text, 64)
	*n = number(f)
	return err
}

// The values no finite number spells, as arguments.
var (
	nan    = number(math.NaN())
	posInf = number(math.Inf(1))
	negInf = number(math.Inf(-1))
)

// step is one operation of a run and the values it takes, each drawn on its own (ADR-0069).
type step struct {
	Op            int
	Advance       int64
	Class         uint8
	Tokens        number
	Cost          number
	RetryAfter    int64
	HasRetryAfter bool
	Draw          number
	Median        number
	Baseline      number
	Value         number
	Expires       int64
	// StaleAt is the instant a corrupted store hands back for issuance's latest instant, taken
	// as that many milliseconds before the clock when Behind is set, so a negative value lies
	// ahead of it.
	StaleAt int64
	Behind  bool
}

// run is one account's history. Kind names the deliberately bad input it carries, and the run is
// otherwise drawn from valid values. Target is the target as a fraction of the ceiling, the default
// or one an account lowered.
type run struct {
	Kind         int
	Ceiling      number
	Target       number
	Start        int64
	Rate         number
	Level        number
	At           int64
	BackoffUntil int64
	Throttles    int
	Steps        []step
}

// The kinds of run, each with the deliberately bad input it carries (ADR-0055, ADR-0069).
var kinds = []string{
	"valid",
	"bad ceiling",
	"bad stored state",
	"rate corrupted during the run",
	"bad retry-after or jitter draw",
	"bad latency",
	"clock running backwards or near its limit",
	"bad request",
	"bad stored lease",
	"bad call cost",
	"throttle storm",
	"idle stretch then a burst",
	"stored level ahead of the clock",
	"clock stepping back then forward again",
	"stored instant come back behind or ahead of the grants",
}

// drawSustained draws requests of one class, large and a few hundred milliseconds apart for three
// seconds or more, so issuance keeps meeting the one-second window. A bucket that holds or refills
// more than it should shows here, because the window alone lets through the hard cap every second,
// which passes the hard cap plus the target times the span after a second or two.
func drawSustained(t *rapid.T, ceiling float64) []step {
	class := rapid.Uint8Range(uint8(core.Interactive), uint8(core.Batch)).Draw(t, "sustained class")
	var run []step
	for range rapid.IntRange(15, 30).Draw(t, "sustained length") {
		s := drawStep(t, ceiling)
		s.Op = opRequest
		s.Class = class
		s.Tokens = number(ceiling * rapid.Float64Range(0.3, 0.8).Draw(t, "sustained tokens as a share of the ceiling"))
		s.Advance = rapid.Int64Range(200, 400).Draw(t, "sustained advance")
		run = append(run, s)
	}
	return run
}

// drawBurst draws requests of one class close together for a few seconds, which drain the bucket
// inside one second and keep drawing on it after. With idle set, a stretch long enough to fill the
// bucket at the floor comes first. A cap that is sized wrong, counted over the wrong window or
// filled twice shows here.
func drawBurst(t *rapid.T, ceiling float64, idle bool) []step {
	class := rapid.Uint8Range(uint8(core.Interactive), uint8(core.Batch)).Draw(t, "burst class")
	var burst []step
	for i := range rapid.IntRange(8, 25).Draw(t, "burst length") {
		s := drawStep(t, ceiling)
		s.Op = opRequest
		s.Class = class
		s.Tokens = number(ceiling * rapid.Float64Range(0.2, 0.8).Draw(t, "burst tokens as a share of the ceiling"))
		s.Advance = rapid.Int64Range(0, 300).Draw(t, "burst advance")
		if i == 0 && idle {
			s.Advance = rapid.Int64Range(35_000, 120_000).Draw(t, "idle stretch")
		}
		burst = append(burst, s)
	}
	return burst
}

// A request's tokens under a ceiling, mostly within the bucket and sometimes past it.
func drawTokens(t *rapid.T, ceiling float64) number {
	return number(ceiling * rapid.Float64Range(0.001, 0.85).Draw(t, "tokens as a share of the ceiling"))
}

// drawStep draws one operation from valid values, with requests the most common. It never draws a
// corrupted instant, which only its own run kind puts in, so the test's copy of the issuer's clock
// stays the issuer's in every other kind.
func drawStep(t *rapid.T, ceiling float64) step {
	op := rapid.IntRange(0, opCorruptInstant+3).Draw(t, "op")
	if op >= opCorruptInstant {
		op = opRequest
	}
	return step{
		Op:            op,
		Advance:       rapid.Int64Range(0, 700).Draw(t, "advance"),
		Class:         rapid.Uint8Range(uint8(core.Interactive), uint8(core.Batch)).Draw(t, "class"),
		Tokens:        drawTokens(t, ceiling),
		Cost:          number(ceiling * rapid.Float64Range(0.001, 0.2).Draw(t, "cost as a share of the ceiling")),
		RetryAfter:    rapid.Int64Range(1, 5000).Draw(t, "retry-after"),
		HasRetryAfter: rapid.Bool().Draw(t, "has retry-after"),
		Draw:          number(rapid.Float64Range(0, 0.999).Draw(t, "jitter draw")),
		Median:        number(rapid.Float64Range(1, 500).Draw(t, "median")),
		Baseline:      number(rapid.Float64Range(1, 500).Draw(t, "baseline")),
		Value:         number(ceiling * rapid.Float64Range(0.001, 0.3).Draw(t, "stored tokens as a share of the ceiling")),
		Expires:       rapid.Int64Range(-2000, 2000).Draw(t, "expiry from the clock"),
	}
}

// drawKind draws a kind with close to equal odds. rapid draws small integers far more often than
// large ones, which starves the kinds late in the list, so the kind is built from coin flips, which
// rapid draws evenly, and a value past the last kind is drawn again.
func drawKind(t *rapid.T) int {
	for {
		kind := 0
		for range 4 {
			kind *= 2
			if rapid.Bool().Draw(t, "kind bit") {
				kind++
			}
		}
		if kind < len(kinds) {
			return kind
		}
	}
}

// drawTarget draws the default target half the time, and otherwise a target an account lowered,
// from just above the floor up to the default (ADR-0024). The lowest is far enough above the floor
// that the four-byte storage of the rate cannot round it onto the floor, which LimitsWithTarget
// refuses.
func drawTarget(t *rapid.T) float64 {
	if rapid.Bool().Draw(t, "target lowered") {
		return rapid.Float64Range(0.0501, 0.5).Draw(t, "lowered target as a share of the ceiling")
	}
	return 0.5
}

func drawRun(t *rapid.T) run {
	kind := drawKind(t)
	ceiling := rapid.SampledFrom([]float64{100, 250, 1, 1e6}).Draw(t, "ceiling")
	r := run{
		Kind:    kind,
		Ceiling: number(ceiling),
		Target:  number(drawTarget(t)),
		Start:   rapid.Int64Range(0, 2_000_000_000_000).Draw(t, "start"),
		Rate:    number(ceiling * rapid.Float64Range(0.05, 0.5).Draw(t, "rate as a share of the ceiling")),
	}
	r.Level = number(ceiling * rapid.Float64Range(0, 0.8).Draw(t, "level as a share of the ceiling"))
	r.At = r.Start
	steps := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) step { return drawStep(t, ceiling) }), 1, 60).Draw(t, "steps")
	var bad []step
	switch kinds[kind] {
	case "bad ceiling":
		// Judged at the default target, since a lowered target under a ceiling past the four-byte
		// range is refused.
		r.Target = 0.5
		// An infinite ceiling is the one that would fix an unbounded budget, so it is drawn half
		// the time and the rest share the other half.
		r.Ceiling = posInf
		if rapid.Bool().Draw(t, "ceiling other than infinite") {
			r.Ceiling = rapid.SampledFrom([]number{0, -250, nan, negInf, math.SmallestNonzeroFloat64, math.MaxFloat64}).Draw(t, "bad ceiling")
		}
		bad = drawBurst(t, ceiling, true)
	case "bad stored state":
		r.Rate = rapid.SampledFrom([]number{nan, -1, 0, posInf, number(10 * ceiling), math.MaxFloat64}).Draw(t, "bad rate")
		r.Level = rapid.SampledFrom([]number{nan, -1, posInf, negInf, 1e300}).Draw(t, "bad level")
		r.At = rapid.SampledFrom([]int64{math.MinInt64, -1, 0, r.Start + 1_000_000, math.MaxInt64}).Draw(t, "bad instant")
		r.BackoffUntil = rapid.SampledFrom([]int64{math.MinInt64, -1, r.Start - 1, r.Start + 3000}).Draw(t, "bad backoff")
		r.Throttles = rapid.SampledFrom([]int{-5, math.MinInt, math.MaxInt}).Draw(t, "bad throttle count")
	case "rate corrupted during the run":
		first := true
		for range rapid.IntRange(1, 3).Draw(t, "corruptions") {
			s := drawStep(t, ceiling)
			s.Op = opCorruptRate
			values := []number{nan, -1, posInf, number(10 * ceiling), math.MaxFloat64}
			if first {
				values = []number{posInf, number(10 * ceiling), math.MaxFloat64}
				first = false
			}
			s.Value = rapid.SampledFrom(values).Draw(t, "bad rate")
			bad = append(bad, s)
			bad = append(bad, drawSustained(t, ceiling)...)
		}
	case "bad retry-after or jitter draw":
		s := drawStep(t, ceiling)
		s.Op = opThrottle
		s.HasRetryAfter = true
		s.RetryAfter = rapid.SampledFrom([]int64{0, -1, math.MinInt64, math.MaxInt64}).Draw(t, "bad retry-after")
		s.Draw = rapid.SampledFrom([]number{nan, -1, 1, 2, posInf}).Draw(t, "bad draw")
		bad = append(bad, s)
	case "bad latency":
		s := drawStep(t, ceiling)
		s.Op = opLatency
		s.Median = rapid.SampledFrom([]number{nan, posInf, -1, 0, 1e300}).Draw(t, "bad median")
		s.Baseline = rapid.SampledFrom([]number{nan, posInf, -1, 0, 1e-300}).Draw(t, "bad baseline")
		bad = append(bad, s)
	case "clock running backwards or near its limit":
		for range rapid.IntRange(1, 4).Draw(t, "clock jumps") {
			s := drawStep(t, ceiling)
			s.Op = opRequest
			s.Advance = rapid.SampledFrom([]int64{-1, -999, -100_000, math.MinInt64, math.MaxInt64}).Draw(t, "bad advance")
			bad = append(bad, s)
		}
	case "bad request":
		s := drawStep(t, ceiling)
		s.Op = opRequest
		s.Class = rapid.SampledFrom([]uint8{0, uint8(core.Batch) + 1, 255, uint8(core.Batch)}).Draw(t, "bad class")
		s.Tokens = rapid.SampledFrom([]number{nan, -1, 0, posInf, 1e300, number(0.8000001 * ceiling)}).Draw(t, "bad tokens")
		bad = append(bad, s)
	case "bad stored lease":
		s := drawStep(t, ceiling)
		s.Op = opStoreLease
		s.Class = rapid.SampledFrom([]uint8{0, uint8(core.Interactive), uint8(core.Sync), 9}).Draw(t, "stored class")
		s.Value = rapid.SampledFrom([]number{nan, -1, -1e300, posInf, negInf, 1e300}).Draw(t, "bad stored tokens")
		s.Expires = rapid.SampledFrom([]int64{math.MinInt64, math.MaxInt64, 0}).Draw(t, "bad stored expiry")
		bad = append(bad, s)
	case "bad call cost":
		s := drawStep(t, ceiling)
		s.Op = opSuccess
		s.Cost = rapid.SampledFrom([]number{nan, -1, 0, posInf, math.MaxFloat64}).Draw(t, "bad cost")
		bad = append(bad, s)
	case "throttle storm":
		for range rapid.IntRange(10, 40).Draw(t, "throttles") {
			s := drawStep(t, ceiling)
			s.Op = opThrottle
			bad = append(bad, s)
		}
	case "idle stretch then a burst":
		for range rapid.IntRange(1, 3).Draw(t, "bursts") {
			bad = append(bad, drawBurst(t, ceiling, true)...)
		}
	case "stored level ahead of the clock":
		// A stored level is held to the capacity by the refill once the clock passes the stored
		// instant, so the instant sits inside the sustained run that follows, which keeps drawing
		// after it.
		r.At = r.Start + rapid.Int64Range(1000, 2500).Draw(t, "stored instant ahead of the clock")
		r.Level = rapid.SampledFrom([]number{1e300, posInf, number(10 * ceiling)}).Draw(t, "stored level past the capacity")
		return r.withSteps(slices.Concat(drawSustained(t, ceiling), steps))
	case "clock stepping back then forward again":
		back := rapid.Int64Range(1000, 60_000).Draw(t, "step back")
		bad = drawBurst(t, ceiling, true)
		s := drawStep(t, ceiling)
		s.Op, s.Advance = opRequest, -back
		bad = append(bad, s)
		burst := drawBurst(t, ceiling, false)
		burst[0].Advance = back
		bad = append(bad, burst...)
	case "stored instant come back behind or ahead of the grants":
		// Grants fill the window, then the store hands back an instant behind them or ahead of the
		// clock with the window and the level whole, and the requests keep coming inside the same
		// second.
		for range rapid.IntRange(1, 3).Draw(t, "stale instants") {
			bad = append(bad, drawBurst(t, ceiling, true)...)
			s := drawStep(t, ceiling)
			s.Op, s.Advance = opCorruptInstant, 0
			s.Behind = rapid.Bool().Draw(t, "stale instant relative to the clock")
			if s.Behind {
				s.StaleAt = rapid.SampledFrom([]int64{1000, 5000, 10_000, -1500, -3_600_000}).Draw(t, "stale instant from the clock")
			} else {
				s.StaleAt = rapid.SampledFrom([]int64{0, -1, math.MinInt64, math.MaxInt64}).Draw(t, "stale instant")
			}
			bad = append(bad, s)
			for range rapid.IntRange(4, 12).Draw(t, "requests after the stale instant") {
				req := drawStep(t, ceiling)
				req.Op = opRequest
				req.Advance = rapid.Int64Range(0, 100).Draw(t, "advance after the stale instant")
				req.Tokens = number(ceiling * rapid.Float64Range(0.1, 0.4).Draw(t, "tokens after the stale instant"))
				bad = append(bad, req)
			}
		}
	}
	at := rapid.IntRange(0, len(steps)).Draw(t, "where the bad input goes")
	return r.withSteps(slices.Concat(steps[:at], bad, steps[at:]))
}

func (r run) withSteps(steps []step) run {
	r.Steps = steps
	return r
}

// clockAdd is saturating addition for the run's clock, written apart from the code under test.
func clockAdd(a, b int64) int64 {
	switch {
	case b > 0 && a > math.MaxInt64-b:
		return math.MaxInt64
	case b < 0 && a < math.MinInt64-b:
		return math.MinInt64
	default:
		return a + b
	}
}

// finitePositive reports whether a ceiling fixes a budget, read from ADR-0024's fractions.
func finitePositive(ceiling float64) bool {
	return ceiling > 0 && !math.IsInf(ceiling, 1) && ceiling*0.05 > 0
}

// grant is a granted lease and the instant on the issuer's clock it was issued at.
type grant struct {
	at     int64
	tokens float64
}

// replay runs r through the rules under the limits l as a shell would, keeping every lease Issue
// returns, and returns the grants in issue order. check is called after every controller rule with
// the state it returned.
func replay(r run, l core.Limits, check func(core.State)) []grant {
	s := core.State{Rate: float64(r.Rate), BackoffUntil: r.BackoffUntil, Throttles: r.Throttles}
	is := core.Issuance{Level: float64(r.Level), At: r.At}
	var leases []core.Lease
	var grants []grant
	clock := r.Start
	issuerClock := r.At
	for _, st := range r.Steps {
		clock = clockAdd(clock, st.Advance)
		switch st.Op {
		case opRequest:
			var d core.Decision
			is, d = core.Issue(l, s, is, leases, core.Request{Class: core.Class(st.Class), Tokens: float64(st.Tokens)}, clock)
			// Issuance's clock is the latest instant of every request it decided, and a refused
			// request is not decided.
			if d.Outcome != core.Refused {
				issuerClock = max(issuerClock, clock)
			}
			if d.Outcome == core.Granted {
				grants = append(grants, grant{at: issuerClock, tokens: d.Lease.Tokens})
				leases = append(leases, d.Lease)
			}
		case opSuccess:
			s = core.Succeeded(s, l, mail.OpCost{Weight: float64(st.Cost), OpsCount: 1})
			check(s)
		case opThrottle:
			signal := mail.ThrottleSignal{RetryAfterMillis: st.RetryAfter, HasRetryAfter: st.HasRetryAfter}
			s = core.Throttled(s, l, signal, clock, float64(st.Draw))
			check(s)
		case opServerError:
			s = core.ServerErrored(s, l)
			check(s)
		case opLatency:
			s = core.LatencyMeasured(s, l, float64(st.Median), float64(st.Baseline))
			check(s)
		case opCorruptRate:
			s.Rate = float64(st.Value)
		case opStoreLease:
			leases = append(leases, core.Lease{Class: core.Class(st.Class), Tokens: float64(st.Value), Expires: clockAdd(clock, st.Expires)})
		case opPrune:
			leases = core.Live(leases, clock)
		case opCorruptInstant:
			is.At = st.StaleAt
			if st.Behind {
				is.At = clockAdd(clock, -st.StaleAt)
			}
		}
	}
	return grants
}

// A postcondition over every run, bad inputs included, with the hard cap 80% of the declared
// ceiling and the target half of it or lowered to any value above the floor (ADR-0024). The tokens
// issued inside any one-second window sum to at most the hard cap, and over a window of any length
// they stay within the hard cap plus the target times its length. The second bound is not asserted
// on a run whose store handed back an instant behind the grants or ahead of the clock. An instant
// behind lets the bucket refill a stretch it already refilled, which the rules cannot tell from
// idle time, while the one-second window still holds. The grants are measured on the run's own
// clock, so an instant ahead is judged by the real second it lands in. Every controller rule
// returns a rate between the floor and the target. TestAValidRequestIsGrantedOnceTheBucketHoldsIt
// pairs this with the check that issuance grants at all, which an issuer granting nothing would
// fail.
func TestNoRunIssuesPastTheHardCap(t *testing.T) {
	property.Check(t, drawRun, func(t rapid.TB, r run) {
		ceiling := float64(r.Ceiling)
		fraction := float64(r.Target)
		l, err := core.LimitsWithTarget(ceiling, fraction)
		if err != nil {
			t.Fatalf("LimitsWithTarget(%v, %v): %v", ceiling, fraction, err)
		}
		bucketBound := kinds[r.Kind] != "stored instant come back behind or ahead of the grants"
		budget := finitePositive(ceiling)
		hardCap, target := 0.0, 0.0
		if budget {
			hardCap, target = 0.8*ceiling, fraction*ceiling
		}
		grants := replay(r, l, func(s core.State) {
			if budget && !(s.Rate >= 0.05*ceiling && s.Rate <= target) {
				t.Fatalf("kind %q: rate %v outside the floor %v and the target %v", kinds[r.Kind], s.Rate, 0.05*ceiling, target)
			}
		})
		slices.SortStableFunc(grants, func(a, b grant) int { return compareInstants(a.at, b.at) })
		for i := range grants {
			sum := 0.0
			for j := i; j < len(grants); j++ {
				sum += grants[j].tokens
				span := float64(grants[j].at) - float64(grants[i].at)
				if span < 1000 && sum > hardCap*(1+1e-9) {
					t.Fatalf("kind %q: %v tokens issued from %d to %d ms, inside one second, past the hard cap %v", kinds[r.Kind], sum, grants[i].at, grants[j].at, hardCap)
				}
				if limit := hardCap + target*span/1000; bucketBound && sum > limit*(1+1e-9) {
					t.Fatalf("kind %q: %v tokens issued from %d to %d ms, past the hard cap plus the target over that span, %v", kinds[r.Kind], sum, grants[i].at, grants[j].at, limit)
				}
			}
		}
	})
}

func compareInstants(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func kindOfRun(r run) string { return kinds[r.Kind] }

// The generator report for the property above. Each minimum catches a kind the generator stops
// producing. The kinds are drawn with close to equal odds, and across forty seeds at the gating 200
// cases, which every run sets, none fell below 2.5 percent. At a smaller case count a kind can draw
// no case at all, and the report then fails.
func TestNoRunIssuesPastTheHardCapMix(t *testing.T) {
	minimums := map[string]float64{}
	for _, k := range kinds {
		minimums[k] = 0.01
	}
	property.Report(t, drawRun, kindOfRun, minimums)
}

func targetOfRun(r run) string {
	if r.Target < 0.5 {
		return "lowered target"
	}
	return "default target"
}

// The generator report of the target the property above draws, so a generator edit that stops
// lowering it is caught. Each is drawn on an even coin flip, less the rare lowered draw that lands
// on the default itself.
func TestNoRunIssuesPastTheHardCapTargetMix(t *testing.T) {
	property.Report(t, drawRun, targetOfRun, map[string]float64{
		"lowered target": 0.3,
		"default target": 0.3,
	})
}

// request is a valid request against an empty bucket, with no class asking before it and no
// backoff.
type request struct {
	Ceiling float64
	Rate    float64
	Class   uint8
	Tokens  float64
	Start   int64
}

func drawRequest(t *rapid.T) request {
	ceiling := rapid.Float64Range(1, 1e5).Draw(t, "ceiling")
	return request{
		Ceiling: ceiling,
		Rate:    ceiling * rapid.Float64Range(0.05, 0.5).Draw(t, "rate as a share of the ceiling"),
		Class:   rapid.Uint8Range(uint8(core.Interactive), uint8(core.Batch)).Draw(t, "class"),
		Tokens:  ceiling * rapid.Float64Range(0.0001, 0.7999).Draw(t, "tokens as a share of the ceiling"),
		Start:   rapid.Int64Range(1_000_000, 2_000_000_000_000).Draw(t, "start"),
	}
}

// The non-empty check paired with the property above (ADR-0055). A valid request no larger than
// the bucket, from an empty bucket with nothing else asking and no backoff, is granted whole once
// the bucket has filled at the rate for long enough to hold it. At the floor that is more than a
// second for a request costing more than a second's worth, which is how the rate recovers.
func TestAValidRequestIsGrantedOnceTheBucketHoldsIt(t *testing.T) {
	property.Check(t, drawRequest, func(t rapid.TB, a request) {
		wait := int64(math.Ceil(a.Tokens/a.Rate*1000)) + 1
		is := core.Issuance{At: a.Start}
		at := a.Start + wait
		_, d := core.Issue(core.LimitsFor(a.Ceiling), core.State{Rate: a.Rate}, is, nil, core.Request{Class: core.Class(a.Class), Tokens: a.Tokens}, at)
		if d.Outcome != core.Granted || d.Lease.Tokens != a.Tokens || d.Lease.Class != core.Class(a.Class) || d.Lease.Expires != at+1000 {
			t.Fatalf("%+v: after %d ms the decision is %+v, want %v tokens granted until %d", a, wait, d, a.Tokens, at+1000)
		}
	})
}

func requestKind(a request) string {
	if a.Tokens > a.Rate {
		return "more than a second's worth at the rate"
	}
	return "within a second's worth at the rate"
}

// The generator report for the property above. Across forty seeds at the gating 200 cases, which
// every run sets, neither kind fell below 24 percent.
func TestAValidRequestIsGrantedOnceTheBucketHoldsItMix(t *testing.T) {
	property.Report(t, drawRequest, requestKind, map[string]float64{
		"more than a second's worth at the rate": 0.05,
		"within a second's worth at the rate":    0.05,
	})
}
