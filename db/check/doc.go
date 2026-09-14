// Package check holds the checks over the data-access library's own files, run as Go tests. They
// read the statement files and the migration chain with PostgreSQL's parser, derive the account-keyed
// tables from the chain, assert that generated packages declare no table types and leave no stale
// file, and, as an integration test, run every statement under each role the import lists admit to
// it.
//
// Every check runs over the real library and over the test library under testdata, whose statement
// files, generated code, import lists and SQL violation files hold what each check must accept or
// refuse. The real library holds no statement file yet, so the test library is what shows each check
// still reports what it exists to refuse. The sqlc.yaml layout check and the refusal of a subsection
// never generated need a configuration sqlc itself would refuse, so they run over testdata/layout,
// which sqlc never reads.
//
// Nothing outside the tests belongs in this package, so the parser never reaches a binary.
package check
