// Package credential is the data-access subsection for an account's sealed credential in the
// account_state table, under the per-account row-level security policy (ADR-0016, ADR-0091). It holds
// the statements that read and write the credential and nothing else, so a role admitted to it is
// admitted to the credential, and a role admitted to the account's other state or to the accounts
// listing is never planned against a statement that reads or writes one.
package credential
