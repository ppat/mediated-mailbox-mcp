// Package ratestate is the data-access subsection for the rate_state and rate_grants tables, the
// shared coordination row per account and every grant of its last second (ADR-0016, ADR-0025). It
// holds the reads of the rate state made to show the rate limiter's state, which the UI's jobs
// endpoint makes for its rate block (docs/UI.md section 17.4). The rate limiter's lease
// statements, which write the rate state, sit one directory down, in db/ratestate/limiter, so a role
// admitted to this subsection is never planned against a statement that writes it.
package ratestate
