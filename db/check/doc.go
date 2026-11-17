// Package check holds the checks over the data-access library's own files, run as Go tests. They
// read the statement files and the migration chain with PostgreSQL's parser, derive the account-keyed
// tables from the chain, assert that generated packages declare no table types and leave no stale
// file, require sqlc to refuse a statement reading a column the chain does not hold, and hold the
// chain's grantees to the placeholder db/migrate replaces. As integration tests against the chain
// applied from empty, they plan every statement an import list admits as the application user, apply
// the chain for a user of the test's own naming, and attempt what the application user must be refused,
// a schema change, an audit row or a policy history row altered, and the operation log reached from
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
