package check_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
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

// checkGrantee is a role the migration chain names other than the placeholder as a grantee, or the
// placeholder standing anywhere but as a grantee, named as a violation file's want annotation states
// it.
const checkGrantee = "grantee"

// checkGrantees holds a migration of the chain to the placeholder db/migrate replaces with the
// application user's name, since the project names no database user (ADR-0048, ADR-0143). Every
// grantee of a grant or a revoke, a default privilege's included, is the placeholder spelled exactly as
// db/migrate matches it, so every grant reaches the configured user and no other. A grant of one role
// to another is refused, since it would make a user act with another role's privileges. Every other
// role a statement names by name is refused, the placeholder's included, such as a new owner, the role
// whose default privileges change, a policy's roles and a schema's authorization, since a name the
// chain fixed would hold only under the test harness's names, and the application user owns nothing
// and runs no DDL. A statement that creates or renames a role, or sets or resets the session's role or
// authorization, is refused whatever it names, a role given as a string included, since the
// deployment creates and names both users and the chain runs as the migrator's user. A role name a
// function reads from a string, such as set_config's, is left to review. A role named by a
// keyword, such as PUBLIC or CURRENT_USER, is refused as a grantee
// and left alone elsewhere, since it names no user the deployment chooses. The placeholder appears in
// the file's text only as such a grantee, in no comment, string, quoted name or other identifier, so
// the replacement changes nothing but a grantee. An occurrence is matched as db/migrate matches it, the
// placeholder bounded by characters that cannot continue an unquoted identifier, and here in any ASCII
// letter case, so a spelling the replacement would miss is refused too.
func checkGrantees(f sqlFile) []finding {
	var out []finding
	grantees := map[int]bool{}
	for _, raw := range f.stmts {
		// A default privilege's grant is a GrantStmt message the statement holds directly rather than a
		// node, so every message of the tree is read, not only the nodes.
		granted := map[*pg.RoleSpec]bool{}
		var named []*pg.RoleSpec
		eachMessage(raw.GetStmt().ProtoReflect(), func(m protoreflect.Message) {
			switch x := m.Interface().(type) {
			case *pg.GrantRoleStmt:
				out = append(out, f.at(raw, -1, checkGrantee, "a grant of a role to another makes a user act with that role's privileges, and the chain grants privileges to %s alone (ADR-0143)", migrate.Placeholder))
			case *pg.GrantStmt:
				for _, grantee := range x.GetGrantees() {
					granted[grantee.GetRoleSpec()] = true
				}
			case *pg.RoleSpec:
				named = append(named, x)
			case *pg.CreateRoleStmt:
				out = append(out, f.at(raw, -1, checkGrantee, "a migration creates no role, since the deployment creates and names both users before the chain runs (ADR-0143)"))
			case *pg.RenameStmt:
				if x.GetRenameType() == pg.ObjectType_OBJECT_ROLE {
					out = append(out, f.at(raw, -1, checkGrantee, "a migration renames no role, since the deployment names both users (ADR-0143)"))
				}
			case *pg.VariableSetStmt:
				if name := strings.ToLower(x.GetName()); name == "role" || name == "session_authorization" {
					out = append(out, f.at(raw, -1, checkGrantee, "a migration sets no session role or authorization, since it runs as the migrator's user, whose name the deployment chooses (ADR-0143)"))
				}
			}
		})
		for _, r := range named {
			at := int(r.GetLocation())
			name := r.GetRoletype() == pg.RoleSpecType_ROLESPEC_CSTRING
			switch {
			case granted[r] && name && r.GetRolename() == migrate.Placeholder && at >= 0 && strings.HasPrefix(f.src[at:], migrate.Placeholder):
				grantees[at] = true
			case granted[r]:
				out = append(out, f.at(raw, r.GetLocation(), checkGrantee, "a grant or a revoke names a grantee other than %s, which db/migrate replaces with the application user's name (ADR-0048)", migrate.Placeholder))
			case name:
				out = append(out, f.at(raw, r.GetLocation(), checkGrantee, "a statement names the role %q, and the chain names a role only as %s in a grant or a revoke, since the project names no database user (ADR-0048, ADR-0143)", r.GetRolename(), migrate.Placeholder))
			}
		}
	}
	// Letters are folded byte for byte, so each offset stays the source's.
	folded := []byte(f.src)
	for i, c := range folded {
		if c >= 'A' && c <= 'Z' {
			folded[i] = c + 'a' - 'A'
		}
	}
	for _, at := range migrate.Occurrences(folded) {
		if !grantees[at] {
			out = append(out, finding{
				file: f.path, line: strings.Count(f.src[:at], "\n") + 1, check: checkGrantee,
				text: fmt.Sprintf("%s stands only as a grantee, since db/migrate replaces it wherever it stands as a word (ADR-0048)", migrate.Placeholder),
			})
		}
	}
	return out
}

// eachMessage visits m and every message under it, whether or not it is a node.
func eachMessage(m protoreflect.Message, visit func(protoreflect.Message)) {
	visit(m)
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap() || fd.Message() == nil:
		case fd.IsList():
			for i := range v.List().Len() {
				eachMessage(v.List().Get(i).Message(), visit)
			}
		default:
			eachMessage(v.Message(), visit)
		}
		return true
	})
}
