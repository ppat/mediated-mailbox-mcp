// Package completion is the data-access subsection for backfill's completion flags in the account_state
// table, under the per-account row-level security policy (ADR-0016, ADR-0017). It holds the statement
// that sets them for backfill, apart from db/accountstate, so a role admitted to read the account's
// state is never planned against a statement that sets them.
package completion
