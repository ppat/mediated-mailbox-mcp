// Package ratelimit is the rate limiter, published as mediated-mailbox-ratelimit. It is a narrow,
// named exception to the rule that shared code is pure, and it keeps the limiter's pure rules and
// its impure lease code together in one component.
//
// The rules sit under core and the lease code under lease. Nothing belongs in this package itself.
package ratelimit
