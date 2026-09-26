// Package accountstate is the data-access subsection for the account_state table, which holds
// everything an account carries apart from its identifier and provider, its sealed credential among
// it, under the per-account row-level security policy (ADR-0016, ADR-0091). It sits apart from the
// accounts subsection so a role admitted to the listing is never planned against a statement that
// reads or writes a credential.
package accountstate
