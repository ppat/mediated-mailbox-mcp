// Package limiter is the data-access subsection for the rate_state and rate_grants tables as the rate
// limiter reads and writes them, the shared coordination row per account and every grant of its last
// second, across processes (ADR-0016, ADR-0025). The rate limiter is a shared library, so these
// statements run as the application user in each program that spends from the budget (ADR-0143).
package limiter
