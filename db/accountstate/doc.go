// Package accountstate is the data-access subsection for the account_state table, which holds
// everything an account carries apart from its identifier and provider, each statement reaching
// only the transaction's account's row (ADR-0135, ADR-0091). It holds the reads of the account's
// state other than its credential, and the recording of the last provider authentication attempt,
// which every importer of the table may record (ADR-0097). The statements that read and write the
// sealed credential sit one directory down, in db/accountstate/credential, since the records forbid
// them to the UI's code, whose list admits this subsection (ADR-0066, ADR-0084, ADR-0143).
package accountstate
