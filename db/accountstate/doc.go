// Package accountstate is the data-access subsection for the account_state table, which holds
// everything an account carries apart from its identifier and provider, under the per-account
// row-level security policy (ADR-0016, ADR-0091). It holds the reads of the account's state other
// than its credential. The statements that read and write the sealed credential sit one directory
// down, in db/accountstate/credential, so a role admitted to this subsection is never planned
// against a statement that reads or writes a credential.
package accountstate
