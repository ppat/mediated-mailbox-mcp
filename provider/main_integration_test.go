//go:build integration

package provider

import (
	"context"
	"crypto/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// spenders returns a pool connecting as the application user, as every program that spends from the
// budget does, so its grants apply to the leases as they do in production.
func spenders(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + postgres.ApplicationUser
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// superuser returns a connection that holds every privilege, to seed and read what the tests set up
// and observe.
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

var unsafeName = regexp.MustCompile(`[^a-z0-9]+`)

// newAccount creates an account named for the test and returns it, so each test, and each run of it,
// has rate state of its own, within the grammar the schema checks.
func newAccount(t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	name := strings.Trim(unsafeName.ReplaceAllString(strings.ToLower(t.Name()), "-"), "-")
	account := "acct-" + strings.TrimRight(name[:min(len(name), 40)], "-") + "-" + strings.ToLower(rand.Text()[:8])
	if _, err := conn.Exec(t.Context(), "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account); err != nil {
		t.Fatal(err)
	}
	return account
}
