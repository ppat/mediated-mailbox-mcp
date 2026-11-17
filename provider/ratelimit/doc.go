// Package ratelimit is the rate limiter's impure lease code. It issues leases from each account's
// rate state and grants in the database, records the outcomes of the calls they pay for, and enacts
// the decisions its pure rules in provider/ratelimit/core return (ADR-0024, ADR-0025). Its leased
// call, Leased, takes a cost and not a mailbox, so the provider library's one wrapper of a mailbox's
// operations spends through it. Call and Body lease the calls of a core/mail Port through it.
//
// It also holds the rate limiter's metrics (ADR-0076). Each spending process emits its own rate, lease
// and throttle series through its Limiter, and the Collector emits each account's series from its
// rate state for the API server, which registers it (ADR-0077).
package ratelimit
