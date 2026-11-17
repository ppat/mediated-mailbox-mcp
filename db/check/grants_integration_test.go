//go:build integration

package check_test

import (
	"context"
	"errors"
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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
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
	// The planner evaluates current_setting('app.account'), which every statement compares the account
	// with, to estimate how many rows match, and the setting raises while no transaction has set it.
	// Any account serves, since planning reads no rows.
	if _, err := tx.Exec(ctx, "SELECT set_config('app.account', 'grant-check', true)"); err != nil {
		return err
	}
	// The simple protocol sends the text as it is, so the server reads $n as parameters of the plan
	// rather than of the EXPLAIN.
	_, err = tx.Exec(ctx, "EXPLAIN (GENERIC_PLAN) "+sql, pgx.QueryExecModeSimpleProtocol)
	return err
}

// grantProblems plans every statement of every subsection an import list names under user. A list
// admitting code to a statement user cannot run is a problem naming the list, the statement and the
// user, found here instead of at run time. One user's grants cannot tell one list's code from
// another's, so every list is planned under the same user, a library's list as any other (ADR-0066,
// ADR-0143).
func grantProblems(t *testing.T, db beginner, lists importLists, user string) []string {
	t.Helper()
	var out []string
	for _, list := range slices.Sorted(maps.Keys(lists)) {
		for _, s := range lists[list] {
			statements := generatedStatements(t, s)
			if len(statements) == 0 {
				out = append(out, fmt.Sprintf("subsection %s has no generated statements, so its grants were not tested", s.name))
			}
			for _, st := range statements {
				if err := explainAs(t.Context(), db, user, st.sql); err != nil {
					out = append(out, fmt.Sprintf("list %q names %s, but user %s cannot run its %s: %v", list, s.name, user, st.name, err))
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

// TestGrantsCoverAdmittedSubsections plans every statement of every subsection any import list names
// under the application user, as the migration chain grants it.
func TestGrantsCoverAdmittedSubsections(t *testing.T) {
	lists, _ := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, grantProblems(t, connect(t), lists, postgres.ApplicationUser))
}

// TestTheChainGrantsTheNameTheMigratorIsGiven applies the chain from empty with the migrator given the
// name of a user the test creates, a name neither the test harness's application user nor the
// placeholder the chain's grants name, and plans every statement an import list admits under that
// user. It then applies the chain with the migrator given a name no role holds, which must fail at the
// chain's first grant, so the grants reach the user the deployment names and no other (ADR-0048,
// ADR-0143).
func TestTheChainGrantsTheNameTheMigratorIsGiven(t *testing.T) {
	ctx := t.Context()
	admin := connect(t)
	chosen := fmt.Sprintf("Chosen App %d", os.Getpid())
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{chosen}.Sanitize()+" LOGIN"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, postgres.URL(t))
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := conn.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		// The databases ChainFor creates hold the role's grants, and their cleanups, registered after
		// this one, run before it and drop them.
		if _, err := conn.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{chosen}.Sanitize()); err != nil {
			t.Error(err)
		}
	})

	url, err := postgres.ChainFor(t, "chosen", migrations.Chain(), chosen)
	if err != nil {
		t.Fatalf("the chain did not apply with its grants made to %q: %v", chosen, err)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	lists, _ := realLibrary.admissions(t, realLibrary.subsections(t))
	requireNoProblems(t, grantProblems(t, conn, lists, chosen))
	var placeholder bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)", migrate.Placeholder).Scan(&placeholder); err != nil || placeholder {
		t.Fatalf("a role holds the placeholder's name, %v, error %v, so a chain applied without the replacement would not fail", placeholder, err)
	}

	absent := fmt.Sprintf("absent_%d", os.Getpid())
	unnamed, err := postgres.ChainFor(t, "unnamed", migrations.Chain(), absent)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42704" || !strings.Contains(pgErr.Message, absent) {
		t.Fatalf("applying the chain for a user no role holds returned %v, want it refused for want of role %s", err, absent)
	}
	stopped, err := pgx.Connect(ctx, unnamed)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stopped.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	// The grants migration's transaction rolled back, so the chain stands at the migration before it.
	var version int64
	if err := stopped.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil || version != 2 {
		t.Errorf("the chain stopped at version %d, error %v, want 2, before the grants migration", version, err)
	}
}

// TestGrantProblemsReported applies the test library's user and schema inside a transaction it rolls
// back, so the user, which belongs to the whole cluster, never reaches another test package. Its import
// lists hold a list and a library's list each naming a subsection whose update the user holds no grant
// for, and lists naming only what the user can run.
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
	lists, _ := testLibrary.admissions(t, testLibrary.subsections(t))
	requireProblems(t, grantProblems(t, tx, lists, "check_fixture_user"), []string{
		`list "library" names counting, but user check_fixture_user cannot run its AddSenderMessages: ERROR: permission denied for table fixture_senders (SQLSTATE 42501)`,
		`list "misaligned" names counting, but user check_fixture_user cannot run its AddSenderMessages: ERROR: permission denied for table fixture_senders (SQLSTATE 42501)`,
	})
}
