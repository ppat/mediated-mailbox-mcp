//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// mediator returns a pool connecting as the mediator's role, under which row-level security applies
// as it does in production.
func mediator(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=mediated_mailbox_mediate"
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// superuser returns a connection that bypasses row-level security, to write the rows the tests
// stage.
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

// must runs a statement and fails the test on an error.
func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccounts creates two accounts named for the test.
func newAccounts(t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	prefix := "acct-" + strings.ToLower(rand.Text()[:8])
	accounts := []string{prefix + "-a", prefix + "-b"}
	for _, account := range accounts {
		must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	}
	return accounts
}

// The draft-to-approved transition is not the mediator's to make. Its role can neither write a plan's
// status or its approval nor insert a plan already approved, so no operation added to the client
// surface could perform it (ADR-0020, ADR-0021, ADR-0030).
func TestTheMediatorCannotApproveAPlan(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn)
	must(t, conn, `INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), $1, 'DRAFT', '{}')`, accounts[0])
	statements := []string{
		"UPDATE reorg_plans SET status = 'APPROVED'",
		"UPDATE reorg_plans SET approved_at = now(), approved_by = 'mediator'",
		"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), current_setting('app.account'), 'APPROVED', '{}')",
	}
	for _, sql := range statements {
		err := func() (err error) {
			tx, err := mediator(t).Begin(t.Context())
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, tx.Rollback(context.Background())) }()
			if _, err := tx.Exec(t.Context(), "SELECT set_config('app.account', $1, true)", accounts[0]); err != nil {
				return err
			}
			_, err = tx.Exec(t.Context(), sql)
			return err
		}()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s as the mediator: %v, want permission denied", sql, err)
		}
	}
}

// The mediator loads the policy of the accounts it serves through the shared policy loader, under its
// own role, whose grant is migration 00007's. With no account served there is no policy to load, and
// the loader's series is never registered.
func TestTheMediatorLoadsPolicyForItsAccounts(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, $2, 'restricted', ARRAY['examplebank.com'], 'operator', 'test')`, accounts[0], accounts[0]+".bank")

	none := prometheus.NewRegistry()
	if err := loadPolicy(t.Context(), mediator(t), nil, none); err != nil {
		t.Errorf("with no account served: %v", err)
	}
	if families, err := none.Gather(); err != nil || len(families) != 0 {
		t.Errorf("with no account served, the registry holds %d series (%v)", len(families), err)
	}

	one := prometheus.NewRegistry()
	if err := loadPolicy(t.Context(), mediator(t), accounts[:1], one); err != nil {
		t.Fatalf("with one account served: %v", err)
	}
	families, err := one.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var failed []float64
	for _, f := range families {
		if f.GetName() == "mediated_mailbox_policyload_reload_failed" {
			for _, m := range f.GetMetric() {
				failed = append(failed, m.GetGauge().GetValue())
			}
		}
	}
	if len(failed) != 1 || failed[0] != 0 {
		t.Errorf("after loading one account's policy, the reload-failure series reads %v, want one series at 0", failed)
	}
}
