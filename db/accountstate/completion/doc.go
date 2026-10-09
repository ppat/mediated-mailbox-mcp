// Package completion is the data-access subsection for backfill's completion flags in the account_state
// table, under the per-account row-level security policy (ADR-0016, ADR-0017). It holds the
// statements that set them for backfill and clear them when a pass is due again, and those that set,
// read and clear the mark that starts the second pass over (ADR-0120). They sit apart from
// db/accountstate, so a role admitted to read the account's state is never planned against a
// statement that sets them.
package completion
