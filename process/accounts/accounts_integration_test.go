//go:build integration

package accounts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/process/accounts"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// pool returns a pool connecting as the application user every program connects as, so its grants
// apply as they do in production (ADR-0143).
func pool(t *testing.T) *pgxpool.Pool {
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

// VERIFICATIONS' rows for reading the accounts, for the reading itself. A read lists every account in
// identifier order with its provider and the OAuth client its row names, none for one that names
// none. A read that succeeds with no account is the empty list, and a read that fails is a failure
// wrapping ErrUntrustedRead with no account, never an empty list, so the Session keeps the list in
// force (ADR-0090, ADR-0091, ADR-0106).
func TestAReadThatFailsIsNeverAListWithNoAccount(t *testing.T) {
	conn := superuser(t)
	must(t, conn, "DELETE FROM accounts")
	must(t, conn, "DELETE FROM oauth_clients")
	db := pool(t)

	got, err := accounts.Read(t.Context(), db)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("a read of no account returned %v, %v, want the empty list", got, err)
	}

	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'id', 'secret')")
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('work', 'gmail', 'household')")
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ('personal', 'fastmail')")
	got, err = accounts.Read(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	want := []accounts.Account{{ID: "personal", Provider: "fastmail"}, {ID: "work", Provider: "gmail", Client: "household"}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the accounts read (-want +got):\n%s", diff)
	}

	must(t, conn, "REVOKE SELECT ON accounts FROM "+postgres.ApplicationUser)
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), postgres.URL(t))
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(context.Background()); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(context.Background(), "GRANT SELECT ON accounts TO "+postgres.ApplicationUser); err != nil {
			t.Error(err)
		}
	})
	got, err = accounts.Read(t.Context(), db)
	if !errors.Is(err, accounts.ErrUntrustedRead) || got != nil {
		t.Errorf("a read refused its grant returned %v, %v, want no list and ErrUntrustedRead", got, err)
	}
}
