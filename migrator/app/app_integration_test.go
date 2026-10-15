//go:build integration

package app_test

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	neturl "net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/migrator/app"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// TestTheMigratorAppliesTheChainItCarries runs the migrator as the migration step runs it, configured
// through the environment layer alone, with the password in a mounted file and the driver's own
// variables naming another server. Against an empty database it applies every migration of the chain
// its binary carries, and run again it applies nothing and succeeds (ADR-0078, ADR-0079, ADR-0139).
func TestTheMigratorAppliesTheChainItCarries(t *testing.T) {
	ctx := t.Context()
	admin := os.Getenv(postgres.EnvAdminURL)
	if admin == "" {
		t.Fatalf("run under pgrun, which sets %s", postgres.EnvAdminURL)
	}
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("migrator_%d", os.Getpid())
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	u, err := neturl.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	path := filepath.Join(t.TempDir(), "password")
	//nolint:gosec // The path is in the test's own directory.
	if err := os.WriteFile(path, []byte(password+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environ := []string{
		"MEDIATED_MAILBOX_DATABASE__HOST=" + u.Hostname(),
		"MEDIATED_MAILBOX_DATABASE__PORT=" + u.Port(),
		"MEDIATED_MAILBOX_DATABASE__NAME=" + name,
		"MEDIATED_MAILBOX_DATABASE__USER=" + u.User.Username(),
		"MEDIATED_MAILBOX_DATABASE__SSLMODE=disable",
		"MEDIATED_MAILBOX_DATABASE__PASSWORD_FILE=" + path,
		"PGHOST=elsewhere.invalid",
		"PGDATABASE=elsewhere",
	}
	for run := 1; run <= 2; run++ {
		if err := app.Run(ctx, nil, environ, io.Discard, io.Discard, slog.New(slog.NewTextHandler(os.Stderr, nil)), new(slog.LevelVar)); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	var want int64
	if err := fs.WalkDir(migrations.Chain(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var v int64
		if _, err := fmt.Sscanf(path, "%d_", &v); err != nil {
			return err
		}
		want = max(want, v)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	database := *u
	database.Path = "/" + name
	migrated, err := pgx.Connect(ctx, database.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := migrated.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var applied, recorded int64
	if err := migrated.QueryRow(ctx, "SELECT max(version_id), count(*) FILTER (WHERE version_id > 0) FROM goose_db_version").Scan(&applied, &recorded); err != nil {
		t.Fatal(err)
	}
	if files, err := fs.Glob(migrations.Chain(), "*.sql"); err != nil || applied != want || recorded != int64(len(files)) {
		t.Errorf("the database stands at version %d with %d migrations recorded, want %d with each of the chain's %d recorded once (%v)", applied, recorded, want, len(files), err)
	}
}
