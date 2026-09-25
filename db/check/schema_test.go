package check_test

import (
	"maps"
	"slices"
	"testing"

	pg "github.com/pganalyze/pg_query_go/v6"
)

// accountColumn is the column that makes a table account-keyed (ADR-0016, ADR-0047).
const accountColumn = "account_id"

// predicateExceptions names the account-keyed tables a statement may reach without an account
// predicate of its own, each with its reason (ADR-0047). A table with no account column needs no
// entry, because the derivation never lists it. Every entry must name an account-keyed table in the
// chain, so an entry left behind by a dropped table or column fails TestPredicateExceptions.
var predicateExceptions = map[string]string{}

// nullAccountTables names the account-keyed tables whose rows with a null account every account
// inherits, each with its reason. A statement on one of them may state its account predicate as
// ADR-0047 does, the account column equal to a parameter or null, and nothing looser. Every entry
// must name an account-keyed table in the chain, as predicateExceptions' entries must.
var nullAccountTables = map[string]string{
	"policy_rules": "a base rule carries no account and every account inherits it (ADR-0004, ADR-0047)",
}

// column is one column of a table, as the migration chain declares it.
type column struct {
	typ   string // the type's unqualified name, such as citext
	array bool
}

type table struct {
	columns    map[string]column
	primaryKey []string
}

// schema is the chain's tables after every migration has applied, read from the migration files
// rather than from a database.
type schema map[string]*table

// readSchema replays the table-shaping statements of the migration chain in file order. Statements
// that do not change which tables and columns exist are ignored.
func readSchema(t *testing.T, files []sqlFile) schema {
	t.Helper()
	s := schema{}
	for _, f := range files {
		for _, raw := range f.stmts {
			n := raw.GetStmt()
			switch {
			case n.GetCreateStmt() != nil:
				c := n.GetCreateStmt()
				tb := &table{columns: map[string]column{}}
				for _, elt := range c.GetTableElts() {
					if def := elt.GetColumnDef(); def != nil {
						tb.addColumn(def)
					}
					if con := elt.GetConstraint(); con != nil && con.GetContype() == pg.ConstrType_CONSTR_PRIMARY {
						tb.primaryKey = names(con.GetKeys())
					}
				}
				s[c.GetRelation().GetRelname()] = tb
			case n.GetAlterTableStmt() != nil:
				a := n.GetAlterTableStmt()
				tb := s[a.GetRelation().GetRelname()]
				if tb == nil {
					continue
				}
				for _, cmd := range a.GetCmds() {
					c := cmd.GetAlterTableCmd()
					if c.GetSubtype() == pg.AlterTableType_AT_AddColumn {
						tb.addColumn(c.GetDef().GetColumnDef())
					}
					if c.GetSubtype() == pg.AlterTableType_AT_DropColumn {
						delete(tb.columns, c.GetName())
					}
					if con := c.GetDef().GetConstraint(); c.GetSubtype() == pg.AlterTableType_AT_AddConstraint && con.GetContype() == pg.ConstrType_CONSTR_PRIMARY {
						tb.primaryKey = names(con.GetKeys())
					}
				}
			case n.GetRenameStmt() != nil:
				r := n.GetRenameStmt()
				old := r.GetRelation().GetRelname()
				if r.GetRenameType() == pg.ObjectType_OBJECT_TABLE {
					s[r.GetNewname()] = s[old]
					delete(s, old)
				}
				if tb := s[old]; r.GetRenameType() == pg.ObjectType_OBJECT_COLUMN && tb != nil {
					tb.columns[r.GetNewname()] = tb.columns[r.GetSubname()]
					delete(tb.columns, r.GetSubname())
				}
			case n.GetDropStmt() != nil && n.GetDropStmt().GetRemoveType() == pg.ObjectType_OBJECT_TABLE:
				for _, obj := range n.GetDropStmt().GetObjects() {
					qualified := names(obj.GetList().GetItems())
					delete(s, qualified[len(qualified)-1])
				}
			}
		}
	}
	return s
}

func (tb *table) addColumn(def *pg.ColumnDef) {
	typeNames := names(def.GetTypeName().GetNames())
	tb.columns[def.GetColname()] = column{
		typ:   typeNames[len(typeNames)-1],
		array: len(def.GetTypeName().GetArrayBounds()) > 0,
	}
	for _, con := range def.GetConstraints() {
		if con.GetConstraint().GetContype() == pg.ConstrType_CONSTR_PRIMARY {
			tb.primaryKey = []string{def.GetColname()}
		}
	}
}

// accountKeyed derives the account-keyed tables from the schema. It is never kept by hand.
func (s schema) accountKeyed() []string {
	var out []string
	for name, tb := range s {
		if _, ok := tb.columns[accountColumn]; ok {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// staleExceptions returns the exception entries that name no account-keyed table.
func staleExceptions(s schema, exceptions map[string]string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(exceptions)) {
		if !slices.Contains(s.accountKeyed(), name) {
			out = append(out, name)
		}
	}
	return out
}

// migrationChain parses the library's migration files in the order they apply.
func (lib library) migrationChain(t *testing.T) []sqlFile {
	t.Helper()
	var files []sqlFile
	for _, dir := range lib.migrations {
		for _, path := range sqlFiles(t, dir) {
			files = append(files, parseFile(t, path))
		}
	}
	if len(files) == 0 {
		t.Fatal("no migration files found, so every check over them would pass having read nothing")
	}
	return files
}

// TestAccountKeyedTablesDerived checks the derivation against the test library's chain, which holds one
// table with an account column and one without.
func TestAccountKeyedTablesDerived(t *testing.T) {
	got := readSchema(t, testLibrary.migrationChain(t)).accountKeyed()
	if !slices.Contains(got, "fixture_senders") || slices.Contains(got, "fixture_log") {
		t.Errorf("account-keyed tables = %v, want fixture_senders and not fixture_log", got)
	}
}

func TestPredicateExceptions(t *testing.T) {
	s := readSchema(t, realLibrary.migrationChain(t))
	if stale := staleExceptions(s, predicateExceptions); len(stale) > 0 {
		t.Errorf("predicate exceptions naming no account-keyed table: %v", stale)
	}
	if stale := staleExceptions(s, nullAccountTables); len(stale) > 0 {
		t.Errorf("tables taking the account-or-null predicate that are not account-keyed: %v", stale)
	}
}

// TestStaleExceptionsReported shows the stale-entry check refusing entries, since neither real list
// holds a stale one. predicateExceptions is empty until a statement reaches a table ADR-0047 exempts
// outright, and nullAccountTables names policy_rules, which is account-keyed.
func TestStaleExceptionsReported(t *testing.T) {
	s := schema{
		"keyed":   {columns: map[string]column{accountColumn: {typ: "text"}}},
		"unkeyed": {columns: map[string]column{"seq": {typ: "int8"}}},
	}
	got := staleExceptions(s, map[string]string{"keyed": "reason", "unkeyed": "reason", "dropped": "reason"})
	if want := []string{"dropped", "unkeyed"}; !slices.Equal(got, want) {
		t.Errorf("stale exceptions = %v, want %v", got, want)
	}
}
