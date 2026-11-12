package check_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
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

// The compatibility declaration of ADR-0140, named as a violation file's want annotation states them.
const (
	// checkUndeclared is a migration after the baseline that declares neither that every chain the
	// floor admits runs on it nor a floor.
	checkUndeclared = "compatibility-undeclared"
	// checkFloorStatement is a statement on the compatibility floor other than the one form that
	// raises it.
	checkFloorStatement = "floor-statement"
)

// baselineVersion is the version of the baseline's last file, db/README.md's three files that build
// the schema from empty. No older chain runs on a schema the baseline builds, so the baseline declares
// nothing.
const baselineVersion = 3

// compatibleMarker is the line a migration after the baseline holds when the binaries of every chain
// at or above the current floor run on the schema it leaves, since none depends on what it removes or
// changes, so the floor stays (ADR-0140).
const compatibleMarker = "-- compatibility: every chain the floor admits runs on this migration"

// floorTable and floorColumn are the compatibility floor's table and its one column.
const (
	floorTable  = "schema_compatibility"
	floorColumn = "oldest_chain_version"
)

// checkCompatibility requires every migration after the baseline to declare which binaries run on the
// schema it leaves, by the marker line saying every chain the floor admits does, or by raising the floor
// in a statement of the one form UPDATE schema_compatibility SET oldest_chain_version =
// greatest(oldest_chain_version, E), where E is an integer from 1 up to the migration's own version, so
// the floor is never lowered and never set above the chain that raises it. A statement that names the
// floor's table anywhere in its parse tree is admitted only when it is that raise, a read, or a grant or
// a revoke of SELECT alone, and every other is refused, whatever kind of statement it is, so a kind this
// check does not know fails closed. The version is the number the file's name starts with, as goose
// reads it (ADR-0140).
func checkCompatibility(f sqlFile) []finding {
	var version int64
	if _, err := fmt.Sscanf(filepath.Base(f.path), "%d_", &version); err != nil {
		return []finding{{file: f.path, line: 1, check: checkUndeclared, text: "the file is not named for its version, so its declaration cannot be read"}}
	}
	if version <= baselineVersion {
		return nil
	}
	var out []finding
	raised := false
	for _, raw := range f.stmts {
		stmt := raw.GetStmt()
		switch {
		case raisesFloor(stmt, version):
			raised = true
		case !namesIn(stmt.ProtoReflect(), floorTable), readsOnly(stmt, floorTable), grantsSelectOnly(stmt):
		default:
			out = append(out, f.at(raw, -1, checkFloorStatement, "a statement naming the compatibility floor's table is UPDATE %s SET %s = greatest(%s, E), with E from 1 to the migration's version %d, a read, or a grant or a revoke of SELECT alone (ADR-0140)", floorTable, floorColumn, floorColumn, version))
		}
	}
	declared := slices.ContainsFunc(strings.Split(f.src, "\n"), func(line string) bool {
		return strings.TrimSpace(line) == compatibleMarker
	})
	if !declared && !raised {
		out = append(out, finding{file: f.path, line: 1, check: checkUndeclared, text: fmt.Sprintf("a migration after the baseline holds the line %q or raises the compatibility floor (ADR-0140)", compatibleMarker)})
	}
	return out
}

// namesIn reports whether the table's name appears anywhere in a parse tree, as any string-typed field
// equal to it, which covers a range variable's relation name, a name part such as a drop's object list
// holds, and a name a statement gives, such as a rename's new name.
func namesIn(m protoreflect.Message, table string) bool {
	found := false
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
		case fd.Kind() == protoreflect.StringKind && fd.IsList():
			for i := range v.List().Len() {
				found = found || v.List().Get(i).String() == table
			}
		case fd.Kind() == protoreflect.StringKind:
			found = v.String() == table
		case fd.Message() == nil:
		case fd.IsList():
			for i := range v.List().Len() {
				found = found || namesIn(v.List().Get(i).Message(), table)
			}
		default:
			found = namesIn(v.Message(), table)
		}
		return !found
	})
	return found
}

// readsOnly reports whether a statement is a select, or a create table as a select, that writes,
// locks or creates nothing on the table: its parse tree holds no insert, update, delete or merge, no
// row locking clause, and no target named for the table.
func readsOnly(stmt *pg.Node, table string) bool {
	var into *pg.IntoClause
	switch {
	case stmt.GetSelectStmt() != nil:
		into = stmt.GetSelectStmt().GetIntoClause()
	case stmt.GetCreateTableAsStmt() != nil && stmt.GetCreateTableAsStmt().GetQuery().GetSelectStmt() != nil:
		into = stmt.GetCreateTableAsStmt().GetInto()
	default:
		return false
	}
	if into.GetRel().GetRelname() == table {
		return false
	}
	writes := false
	walk(stmt, func(n *pg.Node) bool {
		if n.GetInsertStmt() != nil || n.GetUpdateStmt() != nil || n.GetDeleteStmt() != nil ||
			n.GetMergeStmt() != nil || n.GetLockingClause() != nil {
			writes = true
		}
		return !writes
	})
	return !writes
}

// grantsSelectOnly reports whether a statement is a grant or a revoke on tables whose every named
// privilege is SELECT. A grant naming no privilege is ALL, so it is not one.
func grantsSelectOnly(stmt *pg.Node) bool {
	g := stmt.GetGrantStmt()
	if g == nil || g.GetObjtype() != pg.ObjectType_OBJECT_TABLE || len(g.GetPrivileges()) == 0 {
		return false
	}
	for _, p := range g.GetPrivileges() {
		if p.GetAccessPriv().GetPrivName() != "select" {
			return false
		}
	}
	return true
}

// raisesFloor reports whether a statement is exactly UPDATE schema_compatibility SET
// oldest_chain_version = greatest(oldest_chain_version, E), with no other clause, and E an integer
// from 1 up to version.
func raisesFloor(stmt *pg.Node, version int64) bool {
	u := stmt.GetUpdateStmt()
	if u == nil || u.GetRelation().GetRelname() != floorTable || u.GetWhereClause() != nil ||
		len(u.GetFromClause()) > 0 || len(u.GetReturningList()) > 0 || u.GetWithClause() != nil ||
		len(u.GetTargetList()) != 1 {
		return false
	}
	target := u.GetTargetList()[0].GetResTarget()
	if target.GetName() != floorColumn || len(target.GetIndirection()) > 0 {
		return false
	}
	greatest := target.GetVal().GetMinMaxExpr()
	if greatest == nil || greatest.GetOp() != pg.MinMaxOp_IS_GREATEST || len(greatest.GetArgs()) != 2 {
		return false
	}
	column := greatest.GetArgs()[0].GetColumnRef().GetFields()
	if len(column) != 1 || column[0].GetString_().GetSval() != floorColumn {
		return false
	}
	value := greatest.GetArgs()[1].GetAConst()
	if value == nil || value.GetIval() == nil {
		return false
	}
	e := int64(value.GetIval().GetIval())
	return e >= 1 && e <= version
}
