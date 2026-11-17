package core

import "math"

// leaseMillis is how long a lease counts against its class's share, one second (ADR-0025).
const leaseMillis = 1000

// Issuance is what lease issuance keeps between requests for one account.
type Issuance struct {
	// Level is the tokens in the bucket as of At.
	Level float64
	// At is the latest instant issuance has seen. Issuance takes the later of it and the instant
	// it is given, so its clock never runs backwards and no stretch of time fills the bucket twice.
	At int64
	// Asked holds, indexed by class, the latest instant each class asked for a lease.
	Asked [Batch + 1]int64
	// Recent holds the grants issued within the last second, which the one-second window counts.
	// It must hold every one of them, stored whole beside Level. A grant missing from it lets
	// issuance pass the hard cap, and the rules cannot see the absence (ADR-0024).
	Recent []Issued
}

// Issued is one grant's amount and the instant it was issued at.
type Issued struct {
	At     int64
	Tokens float64
}

// Lease is tokens drawn from the bucket for one class.
type Lease struct {
	Class  Class
	Tokens float64
	// Expires is the instant the lease stops counting against its class's share, one second after
	// it was issued.
	Expires int64
}

// Request is a worker asking for tokens for one class, at least the cost of the call they are for.
type Request struct {
	Class  Class
	Tokens float64
}

// Outcome is what became of a request. The zero value is a refusal.
type Outcome uint8

const (
	// Refused is a request that can never be granted. It names no class, asks for no tokens, or
	// asks for more than one second's worth at the hard cap.
	Refused Outcome = iota
	// Waiting is a request the bucket cannot grant yet.
	Waiting
	// Granted is a request drawn from the bucket.
	Granted
)

// Decision is the outcome of a request, and its lease when it was granted.
type Decision struct {
	Outcome Outcome
	Lease   Lease
}

// Issue decides a request at the instant now under the limits l, given the controller's state, what
// issuance kept and the leases already issued. It returns what issuance keeps next and the decision.
//
// Tokens come from a bucket that refills at the current rate, held at the target, and holds at most
// one second's worth at the hard cap. Beside the bucket, the tokens issued inside any one-second
// window sum to at most the hard cap. So the hard cap holds as a rate per second, and over a longer
// window issuance stays within the target times its length plus one second's worth at the hard cap,
// whatever rate the state holds. The bucket does not fill during a backoff, and nothing is issued
// during one. A request is granted whole or not at all.
//
// A class outranks the classes after it, interactive first and batch last. A request may draw
// everything in the bucket except what each class that outranks it and asked within the last lease
// period has still to draw of its share. A class that did not ask within that period is idle, and
// its share is lent. A lease counts against its class's share until it expires.
func Issue(l Limits, s State, is Issuance, leases []Lease, req Request, now int64) (Issuance, Decision) {
	capacity := l.hardCap
	if !req.Class.named() || !(req.Tokens > 0) || req.Tokens > capacity {
		return is, Decision{}
	}
	asked := now
	now = max(now, is.At)
	rate := refillRate(s.Rate, l)
	is = refill(is, s, rate, capacity, now)
	is.Recent = withinASecond(is.Recent, asked, now)
	is.Asked[req.Class] = now
	if now < s.BackoffUntil {
		return is, Decision{Outcome: Waiting}
	}
	available := is.Level
	for c := Interactive; c < req.Class; c++ {
		if addSaturating(is.Asked[c], leaseMillis) > now {
			available -= max(Share(c, rate, l.target)-held(leases, c, now), 0)
		}
	}
	if req.Tokens > available || windowed(is.Recent)+req.Tokens > l.hardCap {
		return is, Decision{Outcome: Waiting}
	}
	is.Level -= req.Tokens
	is.Recent = append(is.Recent, Issued{At: now, Tokens: req.Tokens})
	return is, Decision{
		Outcome: Granted,
		Lease:   Lease{Class: req.Class, Tokens: req.Tokens, Expires: addSaturating(now, leaseMillis)},
	}
}

// refillRate returns the rate the bucket refills at, the state's rate held at the target. A rate
// that is not a positive number refills nothing.
func refillRate(rate float64, l Limits) float64 {
	if !(rate > 0) {
		return 0
	}
	return min(rate, l.target)
}

// refill returns issuance with the bucket filled at the rate from the later of its last instant and
// the end of the backoff up to now, and held at the capacity. A stored level that is not a number or
// is negative is taken as empty.
func refill(is Issuance, s State, rate, capacity float64, now int64) Issuance {
	level := min(max(is.Level, 0), capacity)
	if math.IsNaN(is.Level) {
		level = 0
	}
	if from := max(is.At, s.BackoffUntil); now > from {
		elapsed := now - from
		if elapsed < 0 {
			elapsed = math.MaxInt64
		}
		level = min(level+rate*float64(elapsed)/leaseMillis, capacity)
	}
	is.Level = level
	is.At = now
	return is
}

// withinASecond returns, as a new list, the grants issued less than a second before the request
// at the instant asked, which issuance raised to now. A grant stamped later than now cannot have
// come from Issue, so it is taken as issued at now, which the list then stores. It counts for one
// second from the request that sees it and leaves, rather than filling the window for as long as
// its stamp lies ahead. The stamp is held to now and never to the stored instant, because a stored
// instant that came back behind would move every real grant out of the window. Membership is judged
// against asked rather than now, because a stored instant that came back ahead of the clock raises
// now and would move every real grant out of the window the same way. When the clock itself steps
// back, grants stamped before the step stay in the window until the clock catches up with them. The
// bucket refills nothing over that stretch either, so issuance was already held back.
func withinASecond(recent []Issued, asked, now int64) []Issued {
	var kept []Issued
	for _, g := range recent {
		g.At = min(g.At, now)
		// Taken from the request's instant rather than added to the stamp, so a grant at the
		// clock's limit still counts instead of saturating straight out of the window.
		if g.At > addSaturating(asked, -leaseMillis) {
			kept = append(kept, g)
		}
	}
	return kept
}

// windowed returns the tokens the grants in the window hold. A stored grant of an amount that is
// not a number counts as the most a window can hold, so it blocks the window until it leaves.
func windowed(recent []Issued) float64 {
	total := 0.0
	for _, g := range recent {
		if math.IsNaN(g.Tokens) {
			return math.Inf(1)
		}
		total += counted(g.Tokens)
	}
	return total
}

// held returns the tokens a class's live leases hold.
func held(leases []Lease, c Class, now int64) float64 {
	total := 0.0
	for _, h := range Live(leases, now) {
		if h.Class == c {
			total += counted(h.Tokens)
		}
	}
	return total
}

// Live returns the leases that have not expired at the instant now. A lease stops counting against
// its class's share when it expires, which is how a crashed worker's tokens return to the pool.
// Nothing it drew is put back into the bucket, because tokens a worker spent before it crashed would
// then be issued twice.
func Live(leases []Lease, now int64) []Lease {
	var live []Lease
	for _, h := range leases {
		if h.Expires > now {
			live = append(live, h)
		}
	}
	return live
}

// counted returns the tokens a stored lease counts for. An amount that is not a positive number
// counts as none, so it leaves its class's share as undrawn as it was.
func counted(tokens float64) float64 {
	if tokens > 0 {
		return tokens
	}
	return 0
}

// named reports whether a constant names the class.
func (c Class) named() bool {
	return c >= Interactive && c <= Batch
}
