package check_test

import (
	pg "github.com/pganalyze/pg_query_go/v6"
)

// checkDatabaseCode is the migration lint of ADR-0060, named as a violation file's want annotation
// states it.
const checkDatabaseCode = "database-code"

// checkMigration refuses anything in a migration file that puts code inside the database. A function
// statement also covers procedures. An anonymous DO block runs procedural code in the database even
// though it leaves nothing behind, so it is refused too.
func checkMigration(f sqlFile) []finding {
	var out []finding
	for _, raw := range f.stmts {
		walk(raw.GetStmt(), func(n *pg.Node) bool {
			var what string
			switch {
			case n.GetCreateFunctionStmt() != nil && n.GetCreateFunctionStmt().GetIsProcedure():
				what = "a procedure"
			case n.GetCreateFunctionStmt() != nil:
				what = "a function"
			case n.GetCreateTrigStmt() != nil:
				what = "a trigger"
			case n.GetCreateEventTrigStmt() != nil:
				what = "an event trigger"
			case n.GetDoStmt() != nil:
				what = "a DO block"
			default:
				return true
			}
			out = append(out, f.at(raw, -1, checkDatabaseCode, "%s runs code inside the database (ADR-0060)", what))
			return false
		})
	}
	return out
}
