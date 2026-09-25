package core_test

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// now is the instant the cases ask at. Under the declared ceiling of 250 the target is 125 and the
// hard cap 200, so the bucket and the one-second window hold at most 200. At the target the shares are 37.5 for interactive,
// 25 for sync and 62.5 for batch.
const now = 100_000

// asked returns the instants each class last asked, interactive first.
func asked(interactive, sync, batch int64) [core.Batch + 1]int64 {
	return [core.Batch + 1]int64{core.Interactive: interactive, core.Sync: sync, core.Batch: batch}
}

// recent returns one grant of the window.
func recent(at int64, tokens float64) []core.Issued {
	return []core.Issued{{At: at, Tokens: tokens}}
}

// never is an instant long enough ago that a class that last asked then is idle.
const never = 0

func granted(c core.Class, tokens float64, at int64) core.Decision {
	return core.Decision{Outcome: core.Granted, Lease: core.Lease{Class: c, Tokens: tokens, Expires: at + 1000}}
}

var (
	waiting = core.Decision{Outcome: core.Waiting}
	refused = core.Decision{Outcome: core.Refused}
)

type issued struct {
	Next     core.Issuance
	Decision core.Decision
}

func TestIssue(t *testing.T) {
	full := core.Issuance{Level: 200, At: now}
	atTarget := core.State{Rate: 125}
	cases := []struct {
		name     string
		ceiling  float64
		state    core.State
		issuance core.Issuance
		req      core.Request
		now      int64
		want     issued
	}{
		{
			"a fresh bucket fills to its capacity and grants",
			declared, atTarget,
			core.Issuance{},
			core.Request{Class: core.Batch, Tokens: 50},
			now,
			issued{core.Issuance{Level: 150, At: now, Asked: asked(never, never, now), Recent: recent(now, 50)}, granted(core.Batch, 50, now)},
		},
		{
			"a request of one second's worth at the hard cap is granted from a full bucket",
			declared, atTarget, full,
			core.Request{Class: core.Sync, Tokens: 200},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, now, never), Recent: recent(now, 200)}, granted(core.Sync, 200, now)},
		},
		{
			"a request past one second's worth at the hard cap is refused and changes nothing",
			declared, atTarget, full,
			core.Request{Class: core.Interactive, Tokens: 200.5},
			now,
			issued{full, refused},
		},
		{
			"a request the bucket cannot cover yet waits, and is still asking",
			declared, atTarget,
			core.Issuance{Level: 10, At: now},
			core.Request{Class: core.Batch, Tokens: 20},
			now,
			issued{core.Issuance{Level: 10, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"the bucket refills at the rate",
			declared,
			core.State{Rate: 100},
			core.Issuance{At: now - 400},
			core.Request{Class: core.Batch, Tokens: 40},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 40)}, granted(core.Batch, 40, now)},
		},
		{
			"a rate above the target refills at the target",
			declared,
			core.State{Rate: 1e9},
			core.Issuance{At: now - 200},
			core.Request{Class: core.Batch, Tokens: 25.5},
			now,
			issued{core.Issuance{Level: 25, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"an infinite rate refills at the target",
			declared,
			core.State{Rate: math.Inf(1)},
			core.Issuance{At: now - 200},
			core.Request{Class: core.Batch, Tokens: 25},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 25)}, granted(core.Batch, 25, now)},
		},
		{
			"a rate that is not a number refills nothing",
			declared,
			core.State{Rate: math.NaN()},
			core.Issuance{At: now - 900},
			core.Request{Class: core.Batch, Tokens: 1},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"a negative rate refills nothing",
			declared,
			core.State{Rate: -50},
			core.Issuance{At: now - 900},
			core.Request{Class: core.Batch, Tokens: 1},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"nothing is issued during a backoff, and the bucket does not fill",
			declared,
			core.State{Rate: 100, BackoffUntil: now + 1},
			core.Issuance{Level: 5, At: now - 800},
			core.Request{Class: core.Interactive, Tokens: 1},
			now,
			issued{core.Issuance{Level: 5, At: now, Asked: asked(now, never, never)}, waiting},
		},
		{
			"after a backoff the bucket fills only from its end",
			declared,
			core.State{Rate: 100, BackoffUntil: now - 300},
			core.Issuance{At: now - 5000},
			core.Request{Class: core.Batch, Tokens: 30},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 30)}, granted(core.Batch, 30, now)},
		},
		{
			"an instant earlier than the last is taken as the last, and fills nothing",
			declared, atTarget,
			core.Issuance{Level: 10, At: now},
			core.Request{Class: core.Batch, Tokens: 10},
			now - 5000,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 10)}, granted(core.Batch, 10, now)},
		},
		{
			"a stored level above the capacity is brought down to it",
			declared,
			core.State{},
			core.Issuance{Level: 1e9, At: now},
			core.Request{Class: core.Batch, Tokens: 200},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 200)}, granted(core.Batch, 200, now)},
		},
		{
			"a stored level that is not a number is taken as empty",
			declared,
			core.State{},
			core.Issuance{Level: math.NaN(), At: now},
			core.Request{Class: core.Batch, Tokens: 1},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"a negative stored level is taken as empty",
			declared,
			core.State{},
			core.Issuance{Level: -40, At: now},
			core.Request{Class: core.Batch, Tokens: 1},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			"the window holds the grants of the last second to the hard cap though the bucket holds more",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(now-999, 150)},
			core.Request{Class: core.Batch, Tokens: 60},
			now,
			issued{core.Issuance{Level: 200, At: now, Asked: asked(never, never, now), Recent: recent(now-999, 150)}, waiting},
		},
		{
			"a grant that fits the window is issued beside the others",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(now-999, 150)},
			core.Request{Class: core.Batch, Tokens: 50},
			now,
			issued{core.Issuance{Level: 150, At: now, Asked: asked(never, never, now), Recent: []core.Issued{{At: now - 999, Tokens: 150}, {At: now, Tokens: 50}}}, granted(core.Batch, 50, now)},
		},
		{
			"a grant a second old has left the window",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(now-1000, 150)},
			core.Request{Class: core.Batch, Tokens: 200},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: recent(now, 200)}, granted(core.Batch, 200, now)},
		},
		{
			"a stored grant that is not a number fills the window",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(now-10, math.NaN())},
			core.Request{Class: core.Batch, Tokens: 1},
			now,
			issued{core.Issuance{Level: 200, At: now, Asked: asked(never, never, now), Recent: recent(now-10, math.NaN())}, waiting},
		},
		{
			"a stored grant stamped past the request is taken as issued at the request",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(math.MaxInt64, 150)},
			core.Request{Class: core.Batch, Tokens: 60},
			now + 500,
			issued{core.Issuance{Level: 200, At: now + 500, Asked: asked(never, never, now+500), Recent: recent(now+500, 150)}, waiting},
		},
		{
			"a grant at the clock's limit counts in the window",
			declared, atTarget,
			core.Issuance{Level: 200, At: math.MaxInt64, Recent: recent(math.MaxInt64, 150)},
			core.Request{Class: core.Batch, Tokens: 60},
			math.MaxInt64,
			issued{core.Issuance{Level: 200, At: math.MaxInt64, Asked: asked(never, never, math.MaxInt64), Recent: recent(math.MaxInt64, 150)}, waiting},
		},
		{
			"a stored instant that came back behind the grants leaves them in the window",
			declared, atTarget,
			core.Issuance{Level: 200, At: 0, Recent: recent(now-100, 200)},
			core.Request{Class: core.Batch, Tokens: 25},
			now,
			issued{core.Issuance{Level: 200, At: now, Asked: asked(never, never, now), Recent: recent(now-100, 200)}, waiting},
		},
		{
			"a stored grant of negative tokens counts as none",
			declared, atTarget,
			core.Issuance{Level: 200, At: now, Recent: recent(now-10, -50)},
			core.Request{Class: core.Batch, Tokens: 200},
			now,
			issued{core.Issuance{Level: 0, At: now, Asked: asked(never, never, now), Recent: []core.Issued{{At: now - 10, Tokens: -50}, {At: now, Tokens: 200}}}, granted(core.Batch, 200, now)},
		},
		{
			"a lease expires one second after it is issued, near the clock's limit too",
			declared, atTarget,
			core.Issuance{Level: 200, At: math.MaxInt64 - 10},
			core.Request{Class: core.Batch, Tokens: 1},
			math.MaxInt64 - 10,
			issued{
				core.Issuance{Level: 199, At: math.MaxInt64 - 10, Asked: asked(never, never, math.MaxInt64-10), Recent: recent(math.MaxInt64-10, 1)},
				core.Decision{Outcome: core.Granted, Lease: core.Lease{Class: core.Batch, Tokens: 1, Expires: math.MaxInt64}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, d := core.Issue(core.LimitsFor(c.ceiling), c.state, c.issuance, nil, c.req, c.now)
			if diff := cmp.Diff(c.want, issued{next, d}, compare.Options, cmpopts.EquateNaNs()); diff != "" {
				t.Errorf("Issue (-want +got):\n%s", diff)
			}
		})
	}
}

// A request that can never be granted is refused and changes nothing.
func TestIssueRefuses(t *testing.T) {
	full := core.Issuance{Level: 200, At: now}
	cases := []struct {
		name    string
		ceiling float64
		req     core.Request
	}{
		{"no class", declared, core.Request{Tokens: 1}},
		{"a value past the last class", declared, core.Request{Class: core.Batch + 1, Tokens: 1}},
		{"no tokens", declared, core.Request{Class: core.Batch}},
		{"negative tokens", declared, core.Request{Class: core.Batch, Tokens: -1}},
		{"tokens that are not a number", declared, core.Request{Class: core.Batch, Tokens: math.NaN()}},
		{"infinite tokens", declared, core.Request{Class: core.Batch, Tokens: math.Inf(1)}},
		{"a ceiling of zero", 0, core.Request{Class: core.Interactive, Tokens: 1}},
		{"a ceiling that is not a number", math.NaN(), core.Request{Class: core.Interactive, Tokens: 1}},
		{"an infinite ceiling", math.Inf(1), core.Request{Class: core.Interactive, Tokens: 1}},
		{"a negative ceiling", -250, core.Request{Class: core.Interactive, Tokens: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, d := core.Issue(core.LimitsFor(c.ceiling), core.State{Rate: 125}, full, nil, c.req, now)
			if diff := cmp.Diff(issued{full, refused}, issued{next, d}, compare.Options); diff != "" {
				t.Errorf("Issue (-want +got):\n%s", diff)
			}
		})
	}
}

// The class split applied to issuance. The bucket holds 75, the rate is at the target, and every case
// asks at the same instant, so only which classes asked and what they hold differ.
func TestIssueSplitsByClass(t *testing.T) {
	recently := int64(now - 999)
	lapsed := int64(now - 1000)
	cases := []struct {
		name   string
		rate   float64
		asked  [core.Batch + 1]int64
		leases []core.Lease
		req    core.Request
		want   core.Decision
	}{
		{
			"batch may draw all but what interactive has still to draw of its share",
			125, asked(recently, never, never), nil,
			core.Request{Class: core.Batch, Tokens: 37.5},
			granted(core.Batch, 37.5, now),
		},
		{
			"and no more",
			125, asked(recently, never, never), nil,
			core.Request{Class: core.Batch, Tokens: 37.75},
			waiting,
		},
		{
			"interactive's live lease counts against its share",
			125, asked(recently, never, never),
			[]core.Lease{{Class: core.Interactive, Tokens: 30, Expires: now + 1}},
			core.Request{Class: core.Batch, Tokens: 67.5},
			granted(core.Batch, 67.5, now),
		},
		{
			"interactive's expired lease no longer counts, so its share is reserved again",
			125, asked(recently, never, never),
			[]core.Lease{{Class: core.Interactive, Tokens: 30, Expires: now}},
			core.Request{Class: core.Batch, Tokens: 37.75},
			waiting,
		},
		{
			"another class's lease does not count against interactive's share",
			125, asked(recently, never, never),
			[]core.Lease{{Class: core.Batch, Tokens: 30, Expires: now + 1}},
			core.Request{Class: core.Batch, Tokens: 37.75},
			waiting,
		},
		{
			"a stored lease of negative tokens counts as none",
			125, asked(recently, never, never),
			[]core.Lease{{Class: core.Interactive, Tokens: -30, Expires: now + 1}},
			core.Request{Class: core.Batch, Tokens: 37.75},
			waiting,
		},
		{
			"an idle interactive share is lent",
			125, asked(lapsed, never, never), nil,
			core.Request{Class: core.Batch, Tokens: 75},
			granted(core.Batch, 75, now),
		},
		{
			"interactive does not wait on batch's share",
			125, asked(never, recently, recently), nil,
			core.Request{Class: core.Interactive, Tokens: 75},
			granted(core.Interactive, 75, now),
		},
		{
			"sync waits on interactive's share",
			125, asked(recently, never, never), nil,
			core.Request{Class: core.Sync, Tokens: 37.75},
			waiting,
		},
		{
			"sync does not wait on batch's share",
			125, asked(never, never, recently), nil,
			core.Request{Class: core.Sync, Tokens: 75},
			granted(core.Sync, 75, now),
		},
		{
			"batch waits on both shares",
			125, asked(recently, recently, never), nil,
			core.Request{Class: core.Batch, Tokens: 12.75},
			waiting,
		},
		{
			"batch draws what both shares leave",
			125, asked(recently, recently, never), nil,
			core.Request{Class: core.Batch, Tokens: 12.5},
			granted(core.Batch, 12.5, now),
		},
		{
			// At half the target the shares are 37.5 for interactive, 25 for sync and none for
			// batch, so the halving comes out of batch alone.
			"after a halving interactive's and sync's shares hold whole",
			62.5, asked(recently, recently, never), nil,
			core.Request{Class: core.Batch, Tokens: 12.5},
			granted(core.Batch, 12.5, now),
		},
		{
			// At 30 the shares are 30 for interactive and none for the others.
			"a cut past sync's share comes out of it",
			30, asked(recently, recently, never), nil,
			core.Request{Class: core.Batch, Tokens: 45},
			granted(core.Batch, 45, now),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			is := core.Issuance{Level: 75, At: now, Asked: c.asked}
			_, d := core.Issue(core.LimitsFor(declared), core.State{Rate: c.rate}, is, c.leases, c.req, now)
			if diff := cmp.Diff(c.want, d, compare.Options); diff != "" {
				t.Errorf("Issue (-want +got):\n%s", diff)
			}
		})
	}
}

// Issuance under a target lowered to 50 units a second, a fifth of the declared ceiling, with the
// stored rate at the default target of 125. The bucket refills at the lowered target, a class's
// reserved share is of the lowered target, and the hard cap stays at 200 (ADR-0024, ADR-0025).
func TestIssueUnderALoweredTarget(t *testing.T) {
	l, err := core.LimitsWithTarget(declared, 0.2)
	if err != nil {
		t.Fatalf("LimitsWithTarget: %v", err)
	}
	atDefault := core.State{Rate: 125}
	cases := []struct {
		name     string
		issuance core.Issuance
		req      core.Request
		want     issued
	}{
		{
			"a second refills the lowered target's worth",
			core.Issuance{At: now - 1000},
			core.Request{Class: core.Batch, Tokens: 50},
			issued{core.Issuance{At: now, Asked: asked(never, never, now), Recent: recent(now, 50)}, granted(core.Batch, 50, now)},
		},
		{
			"and no more than that",
			core.Issuance{At: now - 1000},
			core.Request{Class: core.Batch, Tokens: 50.5},
			issued{core.Issuance{Level: 50, At: now, Asked: asked(never, never, now)}, waiting},
		},
		{
			// Interactive's share at the lowered target is 30% of 50, so 60 of the 75 are lent.
			"batch may draw all but interactive's share of the lowered target",
			core.Issuance{Level: 75, At: now, Asked: asked(now-999, never, never)},
			core.Request{Class: core.Batch, Tokens: 60},
			issued{core.Issuance{Level: 15, At: now, Asked: asked(now-999, never, now), Recent: recent(now, 60)}, granted(core.Batch, 60, now)},
		},
		{
			"and no more than the share lends",
			core.Issuance{Level: 75, At: now, Asked: asked(now-999, never, never)},
			core.Request{Class: core.Batch, Tokens: 60.5},
			issued{core.Issuance{Level: 75, At: now, Asked: asked(now-999, never, now)}, waiting},
		},
		{
			"a full bucket still holds one second's worth at the hard cap",
			core.Issuance{Level: 200, At: now},
			core.Request{Class: core.Batch, Tokens: 200},
			issued{core.Issuance{At: now, Asked: asked(never, never, now), Recent: recent(now, 200)}, granted(core.Batch, 200, now)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, d := core.Issue(l, atDefault, c.issuance, nil, c.req, now)
			if diff := cmp.Diff(c.want, issued{next, d}, compare.Options); diff != "" {
				t.Errorf("Issue (-want +got):\n%s", diff)
			}
		})
	}
}

// A crashed worker's lease returns to the pool by expiring, and nothing it drew is put back into
// the bucket. Batch draws 50 and crashes. The lease expires a second later, and at 1600 ms the
// bucket holds only the 40 the rate refilled.
func TestAnExpiredLeaseIsNotRefunded(t *testing.T) {
	s := core.State{Rate: 25}
	is, d := core.Issue(core.LimitsFor(declared), s, core.Issuance{Level: 50, At: now}, nil, core.Request{Class: core.Batch, Tokens: 50}, now)
	if d.Outcome != core.Granted {
		t.Fatalf("the first request was not granted: %+v", d)
	}
	leases := []core.Lease{d.Lease}
	later := int64(now + 1600)
	if _, d := core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Batch, Tokens: 40.5}, later); d.Outcome != core.Waiting {
		t.Errorf("a request for more than the refill was %v, want it waiting", d.Outcome)
	}
	if _, d := core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Batch, Tokens: 40}, later); d.Outcome != core.Granted {
		t.Errorf("a request for the refill was %v, want it granted", d.Outcome)
	}
}

func TestLive(t *testing.T) {
	leases := []core.Lease{
		{Class: core.Batch, Tokens: 1, Expires: now - 1},
		{Class: core.Batch, Tokens: 2, Expires: now},
		{Class: core.Sync, Tokens: 3, Expires: now + 1},
		{Class: core.Interactive, Tokens: 4, Expires: math.MaxInt64},
	}
	want := []core.Lease{
		{Class: core.Sync, Tokens: 3, Expires: now + 1},
		{Class: core.Interactive, Tokens: 4, Expires: math.MaxInt64},
	}
	if diff := cmp.Diff(want, core.Live(leases, now), compare.Options); diff != "" {
		t.Errorf("Live (-want +got):\n%s", diff)
	}
}

// Under Gmail's ceiling of 100 the floor is 5 a second, and a message fetch costs 20. At the floor the fetch is still granted once the bucket has filled for four seconds, so
// the rate can recover.
func TestACallCostingMoreThanASecondAtTheFloorIsGranted(t *testing.T) {
	const ceiling = 100
	s := core.State{Rate: 5}
	is := core.Issuance{At: now}
	var d core.Decision
	for at := int64(now); at <= now+4000; at += 250 {
		is, d = core.Issue(core.LimitsFor(ceiling), s, is, nil, core.Request{Class: core.Batch, Tokens: 20}, at)
		if d.Outcome == core.Granted {
			if at != now+4000 {
				t.Errorf("granted at %d ms, want at 4000 ms", at-now)
			}
			return
		}
	}
	t.Errorf("never granted, last outcome %v", d.Outcome)
}

// ADR-0025's scenario. Batch holds leases for the whole bucket, the rate is cut, and interactive
// asks. Interactive is granted within its reserved share as the bucket refills, and batch waits
// until interactive has drawn it, so the cut comes out of batch.
func TestInteractiveKeepsItsShareThroughACut(t *testing.T) {
	s := core.State{Rate: 125}
	is := core.Issuance{Level: 75, At: now}
	var leases []core.Lease
	is, d := core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Batch, Tokens: 75}, now)
	if d.Outcome != core.Granted {
		t.Fatalf("batch's first request: %+v", d)
	}
	leases = append(leases, d.Lease)
	s.Rate = 62.5 // the controller halves the rate
	at := int64(now + 200)
	is, d = core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Interactive, Tokens: 12.5}, at)
	if d.Outcome != core.Granted {
		t.Fatalf("interactive at 200 ms: %+v, want granted from the 12.5 refilled", d)
	}
	leases = append(leases, d.Lease)
	at += 400
	is, d = core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Batch, Tokens: 1}, at)
	if d.Outcome != core.Waiting {
		t.Fatalf("batch at 600 ms: %+v, want waiting while interactive has 25 of its share to draw", d)
	}
	_, d = core.Issue(core.LimitsFor(declared), s, is, leases, core.Request{Class: core.Interactive, Tokens: 25}, at)
	if d.Outcome != core.Granted {
		t.Errorf("interactive at 600 ms: %+v, want granted", d)
	}
}

// Gmail's ceiling of 100 gives a hard cap of 80 and a target of 50, and a thread fetch costs 40. At
// the target an empty bucket holds the fetch after 800 ms, and it is granted then.
func TestAThreadFetchIsGrantedOnceTheBucketHoldsIt(t *testing.T) {
	const ceiling = 100
	s := core.State{Rate: 50}
	is := core.Issuance{At: now}
	var d core.Decision
	for at := int64(now); at <= now+800; at += 100 {
		is, d = core.Issue(core.LimitsFor(ceiling), s, is, nil, core.Request{Class: core.Interactive, Tokens: 40}, at)
		if d.Outcome == core.Granted {
			if at != now+800 {
				t.Errorf("granted at %d ms, want at 800 ms", at-now)
			}
			return
		}
	}
	t.Errorf("never granted, last outcome %v", d.Outcome)
}

// Under Gmail's ceiling of 100 two batch label changes of 50 each are never issued inside one
// second, because together they pass the hard cap of 80. The bucket holds 75 by the second one, so
// only the window holds it back. It is issued once the first has left the window.
func TestTwoCallsPastTheHardCapAreNotIssuedInOneSecond(t *testing.T) {
	const ceiling = 100
	s := core.State{Rate: 50}
	is, d := core.Issue(core.LimitsFor(ceiling), s, core.Issuance{Level: 80, At: now}, nil, core.Request{Class: core.Batch, Tokens: 50}, now)
	if d.Outcome != core.Granted {
		t.Fatalf("the first call: %+v, want granted", d)
	}
	if _, d := core.Issue(core.LimitsFor(ceiling), s, is, nil, core.Request{Class: core.Batch, Tokens: 50}, now+900); d.Outcome != core.Waiting {
		t.Errorf("the second call at 900 ms: %v, want waiting", d.Outcome)
	}
	if _, d := core.Issue(core.LimitsFor(ceiling), s, is, nil, core.Request{Class: core.Batch, Tokens: 50}, now+1000); d.Outcome != core.Granted {
		t.Errorf("the second call at 1000 ms: %v, want granted", d.Outcome)
	}
}

// A grant stamped at the clock's limit is taken as issued at the first request that sees it, even
// a year after the stamp was stored, and it leaves the window one second after that request.
func TestAFutureStampCountsForOneSecondFromTheRequestThatSeesIt(t *testing.T) {
	const year = 365 * 24 * 3600 * 1000
	s := core.State{Rate: 125}
	is := core.Issuance{Level: 200, At: now, Recent: recent(math.MaxInt64, 150)}
	seen := int64(now + year)
	is, d := core.Issue(core.LimitsFor(declared), s, is, nil, core.Request{Class: core.Batch, Tokens: 60}, seen)
	if d.Outcome != core.Waiting {
		t.Fatalf("a request for 60 beside the 150 at the first sighting: %v, want waiting", d.Outcome)
	}
	if _, d := core.Issue(core.LimitsFor(declared), s, is, nil, core.Request{Class: core.Batch, Tokens: 60}, seen+999); d.Outcome != core.Waiting {
		t.Errorf("at 999 ms after the sighting: %v, want waiting", d.Outcome)
	}
	if _, d := core.Issue(core.LimitsFor(declared), s, is, nil, core.Request{Class: core.Batch, Tokens: 200}, seen+1000); d.Outcome != core.Granted {
		t.Errorf("at 1000 ms after the sighting: %v, want granted", d.Outcome)
	}
}

// A stored instant that comes back ahead of the clock does not move real grants out of the
// one-second window. Two hundred is granted at 10,000 ms from a full bucket, and a request for 100
// at 10,900 ms waits on the window alone, since the bucket holds 112.5 by then. The store then
// hands back the level and the window whole with the instant ahead, and the request for 100 at
// 10,950 ms still waits, so no more than the hard cap of 200 goes out inside the second.
func TestAStoredInstantAheadOfTheClockKeepsTheWindow(t *testing.T) {
	cases := []struct {
		name  string
		ahead int64
	}{
		{"a second and a half ahead", 10_950 + 1500},
		{"an hour ahead", 10_950 + 3_600_000},
		{"at the clock's limit", math.MaxInt64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := core.State{Rate: 125}
			is, d := core.Issue(core.LimitsFor(declared), s, core.Issuance{Level: 200, At: 10_000}, nil, core.Request{Class: core.Batch, Tokens: 200}, 10_000)
			if d.Outcome != core.Granted {
				t.Fatalf("the first request: %+v, want granted", d)
			}
			is, d = core.Issue(core.LimitsFor(declared), s, is, nil, core.Request{Class: core.Batch, Tokens: 100}, 10_900)
			if d.Outcome != core.Waiting || is.Level != 112.5 {
				t.Fatalf("at 10,900 ms: %v with the bucket at %v, want waiting with 112.5", d.Outcome, is.Level)
			}
			is.At = c.ahead
			if _, d := core.Issue(core.LimitsFor(declared), s, is, nil, core.Request{Class: core.Batch, Tokens: 100}, 10_950); d.Outcome != core.Waiting {
				t.Errorf("at 10,950 ms with the stored instant %s: %v, want waiting", c.name, d.Outcome)
			}
		})
	}
}
