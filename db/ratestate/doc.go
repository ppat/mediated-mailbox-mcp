// Package ratestate is the data-access subsection for the rate_state and rate_grants tables, the
// shared coordination row per account and every grant of its last second, which the rate limiter
// reads and writes across processes (ADR-0016, ADR-0025). The rate limiter is a shared library, so
// these statements run under the role of each deployable that spends from the budget (ADR-0075).
package ratestate
