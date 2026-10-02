// Package cursor is the data-access subsection for delta sync's change cursor in the account_state
// table, under the per-account row-level security policy (ADR-0016). It holds the read of the cursor
// with the time it was written and the write of both, which only delta sync makes (ADR-0018,
// ADR-0105). Only delta sync's list admits it.
package cursor
