//go:build integration

package deltasync

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// syncPool returns a pool connecting as the application user, so its grants apply as in production.
func syncPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + postgres.ApplicationUser
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// superuser returns a connection that holds every privilege, to write what the tests read.
func superuser(t *testing.T) *pgx.Conn {
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

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// VERIFICATIONS' row for a job's latest success read from recorded state, delta sync's part. Each
// account's job, as the plan's listing makes it, starts from its latest success as its ticks record
// it, the end of its latest tick that succeeded, so a worker that restarts without ticking leaves it
// ageing. An account whose ticks all failed counts from its earliest tick's start, and one with no
// tick from its ensure (ADR-0119).
func TestEachTickJobStartsFromItsRecordedLatestSuccess(t *testing.T) {
	conn := superuser(t)
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state", "account_state", "accounts",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
	for _, id := range []string{"ticking", "failing", "new"} {
		must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id)
		must(t, conn, "INSERT INTO account_state (account_id) VALUES ($1)", id)
	}
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := func(account, id, state string, started, finished time.Time) {
		must(t, conn, "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at) VALUES ($1, $2, 'sync', 'tick', $3, $4, $5)",
			account, id, state, started, finished)
	}
	run("ticking", "t1", "succeeded", t0, t0.Add(time.Minute))
	run("ticking", "t2", "succeeded", t0.Add(5*time.Minute), t0.Add(6*time.Minute))
	run("ticking", "t3", "failed", t0.Add(10*time.Minute), t0.Add(11*time.Minute))
	run("failing", "f1", "failed", t0.Add(time.Hour), t0.Add(time.Hour+time.Minute))
	run("failing", "f2", "failed", t0.Add(2*time.Hour), t0.Add(2*time.Hour+time.Minute))

	plan := (&DeltaSync{pool: syncPool(t), logger: slog.New(slog.DiscardHandler), seeding: t.Context()}).plan()
	got := map[string]time.Time{}
	for _, id := range []string{"ticking", "failing", "new"} {
		job, err := plan.Job(id)
		if err != nil {
			t.Fatal(err)
		}
		got[id] = job.LastSuccess.UTC()
	}
	want := map[string]time.Time{"ticking": t0.Add(6 * time.Minute), "failing": t0.Add(time.Hour), "new": {}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the latest success each job starts from (-want +got):\n%s", diff)
	}
}
