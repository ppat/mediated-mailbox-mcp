// Package check holds the checks over the data-access library's own files, run as Go tests. They
// read the statement files and the migration chain with PostgreSQL's parser, derive the account-keyed
// tables from the chain, assert that generated packages declare no table types and leave no stale
// file, and require sqlc to refuse a statement reading a column the chain does not hold. As
// integration tests against the chain applied from empty, they run every statement under each role
// the import lists admit to it, a shared library's under the role of each deployable admitting it, and attempt what the runtime roles must be refused, a schema change,
// an audit row altered, a write by the UI beyond its grant, and the operation log reached from
// another account.
//
// Every check runs over the real library and over the test library under testdata, whose statement
// files, generated code, import lists and SQL violation files hold what each check must accept or
// refuse. The real library holds only statements each check accepts, so the test library is what
// shows each check still reports what it exists to refuse. The sqlc.yaml layout check and the refusal of a subsection
// never generated need a configuration sqlc itself would refuse, so they run over testdata/layout,
// which sqlc never reads.
//
// Nothing outside the tests belongs in this package, so the parser never reaches a binary.
package check
