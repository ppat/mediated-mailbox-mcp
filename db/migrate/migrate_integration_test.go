//go:build integration

package migrate_test

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) { postgres.Main(m) }

// slow is a migration after the chain that records each time it runs and holds its transaction open
// long enough for a second run started beside it to find it pending, unless that run waits.
const slow = `-- +goose Up
INSERT INTO lock_runs DEFAULT VALUES;
SELECT pg_sleep(2);
`

// TestTwoRunsAtOnceApplyAMigrationOnce starts two applications of the chain with one migration
// pending, at the same moment, against one database, as two pods of the migration step would. Both
// succeed, and the migration ran once and is recorded once, because the second waits for the first's
// advisory lock and then finds nothing pending (ADR-0129, ADR-0139).
func TestTwoRunsAtOnceApplyAMigrationOnce(t *testing.T) {
	ctx := t.Context()
	url := postgres.URL(t)
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.Exec(ctx, "CREATE TABLE lock_runs (ran_at timestamptz NOT NULL DEFAULT clock_timestamp())"); err != nil {
		t.Fatal(err)
	}
	chain := fstest.MapFS{"99999_slow.sql": {Data: []byte(slow)}}
	if err := fs.WalkDir(migrations.Chain(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := fs.ReadFile(migrations.Chain(), path)
		chain[path] = &fstest.MapFile{Data: text}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	config, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			db := stdlib.OpenDB(*config)
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			<-start
			errs[i] = migrate.Apply(ctx, db, chain, slog.New(slog.NewTextHandler(os.Stderr, nil)))
		})
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("run %d failed: %v", i+1, err)
		}
	}
	var runs, recorded int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM lock_runs").Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = 99999").Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || recorded != 1 {
		t.Errorf("the migration ran %d times and is recorded %d times, want once and once", runs, recorded)
	}
}
