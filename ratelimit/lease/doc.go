// Package lease is the rate limiter's impure lease code. It issues leases from each account's rate
// state and grants in the database, records the outcomes of the calls they pay for, and enacts the
// decisions ratelimit/core returns (ADR-0024, ADR-0025).
//
// It also holds the rate limiter's metrics (ADR-0076). Each spending process emits its own rate, lease
// and throttle series through its Limiter, and the Collector emits each account's series from its
// rate state for the mediator, which registers it (ADR-0077).
package lease
