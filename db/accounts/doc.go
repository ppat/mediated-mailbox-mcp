// Package accounts is the data-access subsection for the accounts table, which holds each account's
// identifier and provider and nothing else, so every role that lists accounts reads all of its rows
// (ADR-0016, ADR-0091). Listing the accounts is the statement deliberately left without an account
// predicate (ADR-0047).
package accounts
