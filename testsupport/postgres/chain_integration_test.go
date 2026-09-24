//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// drifted is a migration that applies cleanly to a database holding a table no migration created,
// which is how a database drifts from its chain, and fails from empty, where the table does not exist.
const drifted = `-- +goose Up
ALTER TABLE drifted_labels ADD COLUMN color text;
`

// TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts adds the drifted migration after
// the real chain. Applied to a database from this run's template, holding the chain plus the drift,
// it succeeds, so the migration is not simply broken. Applied from empty the way pgrun prepares every
// run, it must fail, and at that migration, so the chain before it applied.
func TestTheChainFailsFromEmptyOnAMigrationOnlyTheCurrentShapeAccepts(t *testing.T) {
	ctx := t.Context()
	admin, template := os.Getenv(postgres.EnvAdminURL), os.Getenv(postgres.EnvTemplate)
	if admin == "" || template == "" {
		t.Fatalf("run under pgrun, which sets %s and %s", postgres.EnvAdminURL, postgres.EnvTemplate)
	}
	migrations, err := filepath.Glob(filepath.Join("..", "..", "db", "migrations", "*.sql"))
	if err != nil || len(migrations) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	chain := t.TempDir()
	for _, path := range migrations {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		//nolint:gosec // The name is a checked-in migration's, written into the test's own directory.
		if err := os.WriteFile(filepath.Join(chain, filepath.Base(path)), src, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(chain, "99999_drifted.sql"), []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	var lastReal int64
	if _, err := fmt.Sscanf(filepath.Base(migrations[len(migrations)-1]), "%d_", &lastReal); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	// The superuser connection acting as the migration role, whose password pgrun does not export.
	migrate, err := neturl.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	// A space encoded as a plus sign reaches the server as a plus sign.
	migrate.RawQuery += "&options=" + strings.ReplaceAll(neturl.QueryEscape("-c role="+postgres.MigrationRole), "+", "%20")
	suffix := fmt.Sprint(os.Getpid())
	database := func(name string) string {
		t.Cleanup(func() {
			if _, err := conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
				t.Error(err)
			}
		})
		u := *migrate
		u.Path = "/" + name
		return u.String()
	}

	t.Run("current shape", func(t *testing.T) {
		name := "chain_current_" + suffix
		url := database(name)
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+pgx.Identifier{template}.Sanitize()+" OWNER "+postgres.MigrationRole); err != nil {
			t.Fatal(err)
		}
		current, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		_, err = current.Exec(ctx, "CREATE TABLE drifted_labels (name text PRIMARY KEY)")
		if closeErr := current.Close(context.Background()); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatal(err)
		}
		goose := exec.CommandContext(ctx, "goose", "-dir", chain, "postgres", url, "up")
		goose.Env = append(os.Environ(), "GOOSE_DRIVER=", "GOOSE_DBSTRING=", "GOOSE_MIGRATION_DIR=")
		if out, err := goose.CombinedOutput(); err != nil {
			t.Fatalf("the drifted migration failed against the current shape: %v\n%s", err, out)
		}
	})

	t.Run("from empty", func(t *testing.T) {
		name := "chain_empty_" + suffix
		url := database(name)
		if err := postgres.ApplyChain(ctx, admin, migrate.String(), name, filepath.Join("..", "..", "db", "bootstrap", "extensions.sql"), chain); err == nil {
			t.Fatal("the chain applied from empty with a migration that needs a table no migration creates")
		}
		empty, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := empty.Close(context.Background()); err != nil {
				t.Error(err)
			}
		}()
		var applied int64
		if err := empty.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version").Scan(&applied); err != nil || applied != lastReal {
			t.Errorf("the chain applied from empty up to version %d with error %v, want the real chain's last version %d, so the failure is the drifted migration's", applied, err, lastReal)
		}
	})
}
