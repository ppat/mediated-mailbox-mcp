//go:build integration

package check_test

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// The migrations after the baseline, each tested over rows the chain before it wrote (ADR-0048).
const (
	identifierGrammarVersion = 5
	runAndAuditIndexVersion  = 6
)

// upgraded connects to the database of an upgrade as the migration role, closed when the test ends.
func upgraded(t *testing.T, u *postgres.Upgrade) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), u.URL)
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

// store runs each statement, failing the test on the first that fails.
func store(t *testing.T, conn *pgx.Conn, statements ...string) {
	t.Helper()
	for _, sql := range statements {
		if _, err := conn.Exec(t.Context(), sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
}

// rowsOf returns every row of a table as text, in the order of its first column, so two reads compare
// equal only when every column of every row is unchanged.
func rowsOf(t *testing.T, conn *pgx.Conn, table string) []string {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT t::text FROM "+pgx.Identifier{table}.Sanitize()+" AS t ORDER BY 1")
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// F11's row for the identifier grammar's migration. Over accounts the baseline stored, one inside the
// grammar and one outside it, the migration stops and leaves both rows as they were, rewriting
// nothing, since an account is system-of-record data. With the account outside the grammar removed, it
// applies, and the account inside the grammar is unchanged (ADR-0048).
func TestTheIdentifierGrammarStopsOverAStoredAccountOutsideIt(t *testing.T) {
	ctx := t.Context()
	u := postgres.ChainBefore(t, filepath.Join("..", "migrations"), identifierGrammarVersion)
	conn := upgraded(t, u)
	store(t, conn,
		"INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', 'sealed')",
		"INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('personal', 'gmail', 'household'), ('Jo.Smith', 'gmail', 'household')",
	)
	before := rowsOf(t, conn, "accounts")

	out, err := u.Up(ctx)
	if err == nil {
		t.Fatalf("the grammar's migration applied over an account outside the grammar\n%s", out)
	}
	if !strings.Contains(out, "accounts_account_id_grammar") {
		t.Errorf("the migration stopped, but not on the grammar's check\n%s", out)
	}
	if diff := cmp.Diff(before, rowsOf(t, conn, "accounts"), compare.Options); diff != "" {
		t.Errorf("the stopped migration changed the stored accounts (-before +after):\n%s", diff)
	}
	var version int64
	if err := conn.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil || version != identifierGrammarVersion-1 {
		t.Errorf("after the stopped migration the chain stands at version %d with error %v, want %d", version, err, identifierGrammarVersion-1)
	}

	store(t, conn, "DELETE FROM accounts WHERE account_id = 'Jo.Smith'")
	if out, err := u.Up(ctx); err != nil {
		t.Fatalf("the grammar's migration over accounts inside the grammar: %v\n%s", err, out)
	}
	if diff := cmp.Diff([]string{before[slices.IndexFunc(before, func(r string) bool { return strings.HasPrefix(r, "(personal,") })]}, rowsOf(t, conn, "accounts"), compare.Options); diff != "" {
		t.Errorf("the migrated account (-before +after):\n%s", diff)
	}
	var grammar bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'accounts_account_id_grammar')").Scan(&grammar); err != nil || !grammar {
		t.Errorf("after the migration the grammar's check exists: %v, error %v", grammar, err)
	}
}

// F11's row for the run and audit indexes' migration. Over runs and audit rows the chain before it
// stored, the migration changes no row and builds each index the reads that repeat need (ADR-0048,
// ADR-0016).
func TestTheRunAndAuditIndexesLeaveStoredRowsUnchanged(t *testing.T) {
	ctx := t.Context()
	u := postgres.ChainBefore(t, filepath.Join("..", "migrations"), runAndAuditIndexVersion)
	conn := upgraded(t, u)
	store(t, conn,
		"INSERT INTO accounts (account_id, provider) VALUES ('personal', 'gmail'), ('work', 'gmail')",
		`INSERT INTO job_runs (account_id, run_id, workload, pass, state, resumed_from, started_at, finished_at, heartbeat_at, checkpoint, counters, last_error) VALUES
		('personal', 'r-1', 'backfill', 'pass1', 'succeeded', NULL, '2026-09-01T10:00:00Z', '2026-09-01T11:00:00Z', '2026-09-01T11:00:00Z', '{"page": 3, "of": 3}', '{"pages": 3, "messages": 75}', NULL),
		('personal', 'r-2', 'backfill', 'pass2', 'failed', NULL, '2026-09-01T11:00:00Z', '2026-09-01T11:30:00Z', '2026-09-01T11:30:00Z', '{"page": 1}', '{"pages": 1}', 'throttled'),
		('personal', 'r-3', 'backfill', 'pass2', 'running', 'r-2', '2026-09-01T12:00:00Z', NULL, '2026-09-01T12:05:00Z', '{"page": 2}', '{"pages": 2}', NULL),
		('personal', 'r-4', 'sync', 'tick', 'succeeded', NULL, '2026-09-02T10:00:00Z', '2026-09-02T10:00:01Z', '2026-09-02T10:00:01Z', NULL, '{"added": 2}', NULL),
		('work', 'r-5', 'sync', 'gap_recovery', 'succeeded', NULL, '2026-09-02T10:00:00Z', '2026-09-02T10:01:00Z', '2026-09-02T10:01:00Z', NULL, '{"reconciled": 4}', NULL)`,
		`INSERT INTO audit_log (ts, account_id, actor, action, message_id, sensitivity, rule_ids) VALUES
		('2026-09-03T09:00:00Z', 'personal', 'agent', 'READ_BODY', 'm-1', '{"sender_class": "normal"}', '{}'),
		('2026-09-03T09:01:00Z', 'personal', 'agent', 'DENY_BODY', 'm-2', '{"sender_class": "restricted"}', '{bank}'),
		('2026-09-03T09:02:00Z', 'work', 'agent', 'READ_BODY', 'm-1', NULL, NULL)`,
	)
	runs, audit := rowsOf(t, conn, "job_runs"), rowsOf(t, conn, "audit_log")

	if out, err := u.Up(ctx); err != nil {
		t.Fatalf("the indexes' migration over stored runs and audit rows: %v\n%s", err, out)
	}
	if diff := cmp.Diff(runs, rowsOf(t, conn, "job_runs"), compare.Options); diff != "" {
		t.Errorf("the migration changed the stored runs (-before +after):\n%s", diff)
	}
	if diff := cmp.Diff(audit, rowsOf(t, conn, "audit_log"), compare.Options); diff != "" {
		t.Errorf("the migration changed the stored audit rows (-before +after):\n%s", diff)
	}
	rows, err := conn.Query(ctx, "SELECT indexdef FROM pg_indexes WHERE tablename IN ('job_runs', 'audit_log') AND indexname NOT LIKE '%_pkey' ORDER BY indexdef")
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CREATE INDEX audit_log_account_id_message_id_ts_idx ON public.audit_log USING btree (account_id, message_id, ts DESC)",
		"CREATE INDEX audit_log_account_id_ts_idx ON public.audit_log USING btree (account_id, ts DESC)",
		"CREATE INDEX job_runs_account_id_finished_at_idx ON public.job_runs USING btree (account_id, finished_at)",
		"CREATE INDEX job_runs_account_id_idx ON public.job_runs USING btree (account_id) WHERE (state = 'running'::text)",
		"CREATE INDEX job_runs_account_id_started_at_idx ON public.job_runs USING btree (account_id, started_at DESC)",
		"CREATE INDEX job_runs_account_id_workload_pass_started_at_run_id_idx ON public.job_runs USING btree (account_id, workload, pass, started_at DESC, run_id)",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the run and audit indexes after the migration (-want +got):\n%s", diff)
	}
}
