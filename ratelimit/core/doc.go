// Package core holds the rate limiter's pure rules (ADR-0024, ADR-0025). These are the target, the
// hard cap and the floor as fractions of the declared ceiling, the AIMD controller, backoff after a
// throttle, the latency baseline, the priority-class split, lease issuance from a token bucket and a
// one-second window under the hard cap, and the expiry rule that returns a crashed worker's tokens
// to the pool. Every rule takes the state it decides from and returns the state or decision it
// reached as a value, and the lease code in ratelimit/lease stores and enacts it. How that state is
// stored is the lease code's.
//
// Time arrives as int64 milliseconds, instants as Unix milliseconds as core/mail uses them. Two
// clocks reach the rules. Lease expiry, the bucket, the one-second window and backoff are judged by
// the database's clock, which every process shares, so Issue, Live and Throttled take their instant
// from it. Latency samples are durations the caller measures on its own monotonic clock, and a
// latency window's start and end are read from that clock too. Instant arithmetic saturates at the
// int64 limits, so no sum wraps into the past.
//
// Randomness arrives as a value. The full-jitter backoff multiplies its bound by a draw the caller
// takes uniformly from [0, 1). A draw outside that range, or not a number, is taken as the whole
// bound, the longest wait the draw could have given.
//
// A retry-after is honored as given when it is present and positive. Zero or a negative value,
// which is a time already past, counts as absent, and full jitter applies. A throttle signal's scope
// does not change the decision.
//
// A share its class leaves unused is lent. A class that asked for no lease during the last lease
// period counts as idle, and its share is lent to whichever class asks. An idle class that asks
// again has its share reserved again from that request on.
//
// Bad inputs the rules can check decide nothing and never widen issuance. These are values that are
// not a number, negative, out of range, duplicated, out of order or stamped later than the clock has
// reached. A ceiling that is not a finite positive number fixes no budget, and every request under
// it is refused. A stored rate outside the floor and the target is brought back inside them by
// every controller rule, and a rate that is not a number is taken as the floor. Issue holds the
// bucket's refill at the target whatever rate it is given, because the hard cap exists for a rate
// the controller got wrong. A call cost, a latency median or baseline, or a requested amount that
// is not a finite positive number grows, cuts and grants nothing. A stored bucket level outside the
// bucket's capacity is brought inside it. A stored grant in the one-second window whose amount is
// not a number fills the window until it leaves it, one whose amount is negative counts as none,
// and one stamped later than the request that sees it is taken as issued at that request's
// instant. A stored latest instant ahead of the clock raises the instant issuance keeps, but the
// window is judged from the request's own instant, so every real grant stays in it. A stored lease
// holding an amount that is not a positive number counts as holding none.
//
// A grant missing from the one-second window is not a bad input the rules can see. The window
// counts only the grants it is given, so a record that lost one lets issuance pass the hard cap.
// Keeping every grant of the last second is the store's duty (ADR-0024). A stored latest instant
// that came back behind the true one cannot be told from idle time either. The window still holds
// every grant it is given, so the hard cap holds over every second, but the bucket refills a
// stretch it already refilled, so over a longer window issuance can pass the target times its
// length by up to one bucket. Storing the latest instant with the level and the window is the
// store's duty too.
//
// The pure-core import list governs this directory.
package core
