//go:build integration

package first_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// TestOwnDatabase creates a table no migration creates, in a database that must start without it. If
// two packages shared a database, whichever ran second would find the table, in parallel or not. The
// test holds its database for a moment so the other package's binary can overlap with it.
func TestOwnDatabase(t *testing.T) {
	ctx := t.Context()
	conn, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var applied int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id > 0").Scan(&applied); err != nil || applied == 0 {
		t.Fatalf("the database does not hold the applied migration chain: %d migrations, %v", applied, err)
	}
	var found bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass('isolation_probe') IS NOT NULL").Scan(&found); err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("isolation_probe already exists, so another test package used this database")
	}
	if _, err := conn.Exec(ctx, "CREATE TABLE isolation_probe (at timestamptz NOT NULL DEFAULT clock_timestamp())"); err != nil {
		t.Fatal(err)
	}
	var database string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	t.Logf("database %s, held from %s", database, time.Now().Format(time.StampMilli))
	time.Sleep(2 * time.Second)
	t.Logf("released at %s", time.Now().Format(time.StampMilli))
}
