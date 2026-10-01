// Package authentication is the data-access subsection for the last provider authentication
// attempt in the account_state table, under the per-account row-level security policy (ADR-0016,
// ADR-0097). It holds the statement a deployable that calls a provider records the attempt with,
// apart from db/accountstate, so a role admitted to read the account's state is never planned
// against a statement that writes it.
package authentication
