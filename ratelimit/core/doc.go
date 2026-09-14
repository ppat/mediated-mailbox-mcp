// Package core holds the rate limiter's pure rules. These are the cap applied at lease issuance,
// the priority-class split, the AIMD controller, and the expiry rule that returns a crashed
// worker's tokens to the pool.
//
// The pure-core import list governs this directory.
package core
