//go:build integration

package check_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// statement is one generated statement's SQL, as sqlc sends it.
type statement struct {
	name string
	sql  string
}

// generatedStatements reads the statement constants sqlc wrote into a subsection's package, so the
// test runs exactly the SQL the accessors send, with named parameters already rewritten to $n.
func generatedStatements(t *testing.T, s subsection) []statement {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.sql.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []statement
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			sql, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasPrefix(sql, "-- name: ") {
				return true
			}
			out = append(out, statement{name: strings.Fields(sql)[2], sql: sql})
			return true
		})
	}
	return out
}

// beginner is a connection or a transaction, either of which can open a transaction inside it.
type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// explainAs plans a statement as role without running it. EXPLAIN (GENERIC_PLAN) checks table and
// column privileges, where PREPARE does not, and it accepts $n parameters with no values.
func explainAs(ctx context.Context, db beginner, role, sql string) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(context.Background()); err == nil {
			err = rollbackErr
		}
	}()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
		return err
	}
	// The simple protocol sends the text as it is, so the server reads $n as parameters of the plan
	// rather than of the EXPLAIN.
	_, err = tx.Exec(ctx, "EXPLAIN (GENERIC_PLAN) "+sql, pgx.QueryExecModeSimpleProtocol)
	return err
}

// grantProblems runs every statement of every subsection a component list names under the role roles
// gives that component. An import list admitting a component to a statement its role cannot run is a
// problem naming the list, the statement and the role, found here instead of at run time. A list
// without a role is skipped, because TestComponentRoles refuses it.
func grantProblems(t *testing.T, db beginner, admitted map[string][]subsection, roles map[string]string) []string {
	t.Helper()
	var out []string
	for _, list := range slices.Sorted(maps.Keys(admitted)) {
		role, ok := roles[list]
		if !ok {
			continue
		}
		for _, s := range admitted[list] {
			statements := generatedStatements(t, s)
			if len(statements) == 0 {
				out = append(out, fmt.Sprintf("subsection %s has no generated statements, so its grants were not tested", s.name))
			}
			for _, st := range statements {
				if err := explainAs(t.Context(), db, role, st.sql); err != nil {
					out = append(out, fmt.Sprintf("list %q names %s, but role %s cannot run its %s: %v", list, s.name, role, st.name, err))
				}
			}
		}
	}
	return out
}

func connect(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

func TestGrantsCoverAdmittedSubsections(t *testing.T) {
	admitted, _ := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, grantProblems(t, connect(t), admitted, componentRoles))
}

// TestGrantProblemsReported applies the test library's roles and schema inside a transaction it rolls
// back, so the roles, which belong to the whole cluster, never reach another test package. Its import
// lists hold one list naming a subsection whose update its role holds no grant for, and lists naming
// only what their roles can run.
func TestGrantProblemsReported(t *testing.T) {
	ctx := t.Context()
	tx, err := connect(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	// The real chain is already applied to this package's database.
	setup := slices.Concat(sqlFiles(t, testLibrary.bootstrap), sqlFiles(t, "testdata/migrations"))
	for _, path := range setup {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	admitted, _ := testLibrary.admissions(t, testLibrary.subsections(t))
	requireProblems(t, grantProblems(t, tx, admitted, testRoles), []string{
		`list "misaligned" names counting, but role check_fixture_reader cannot run its AddSenderMessages: ERROR: permission denied for table fixture_senders (SQLSTATE 42501)`,
	})
}
