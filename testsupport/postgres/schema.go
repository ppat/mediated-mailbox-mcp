package postgres

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
)

// SchemaChecks runs a deployable's start check under each of its roles over one database of the
// test's own, built from the chain in dir. It holds every migration below the chain's newest first,
// then the whole chain, then a compatibility floor and a newest applied migration set by hand, and
// requires the refusal each state calls for: a schema that has not applied the chain, and one whose
// floor is above it, refused, and the chain's own schema and a newer one whose floor the chain meets
// accepted (ADR-0140). The chain's newest version is read from dir's file names, apart from the code
// under test. It needs the variables pgrun sets.
func SchemaChecks(t *testing.T, dir string, check func(context.Context, *pgxpool.Pool) error, roles ...string) {
	t.Helper()
	ctx := t.Context()
	k := newestIn(t, dir)
	u := ChainBefore(t, dir, k)
	owner, err := pgx.Connect(ctx, u.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	expect := func(state string, want error) {
		t.Helper()
		for _, role := range roles {
			cfg, err := pgxpool.ParseConfig(u.URL)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			err = check(ctx, pool)
			pool.Close()
			if !errors.Is(err, want) || (want == nil) != (err == nil) {
				t.Errorf("%s, as %s: the check returned %v, want %v", state, role, err, want)
			}
		}
	}
	set := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	expect("every migration below the chain's newest applied", migrations.ErrNotMigrated)
	if err := u.Up(ctx); err != nil {
		t.Fatal(err)
	}
	expect("the whole chain applied", nil)
	set("UPDATE schema_compatibility SET oldest_chain_version = $1", k)
	expect("the floor at the chain's newest", nil)
	set("UPDATE schema_compatibility SET oldest_chain_version = $1", k+1)
	expect("the floor one above the chain's newest", migrations.ErrBelowFloor)
	set("INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)", k+1)
	expect("a newer migration that raised the floor above the chain", migrations.ErrBelowFloor)
	set("UPDATE schema_compatibility SET oldest_chain_version = $1", k)
	expect("a newer migration that left the floor at the chain", nil)
}

// newestIn reads the newest migration's version from the file names in dir.
func newestIn(t *testing.T, dir string) int64 {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no migrations found in %s: %v", dir, err)
	}
	var newest int64
	for _, path := range paths {
		var v int64
		if _, err := fmt.Sscanf(filepath.Base(path), "%d_", &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		newest = max(newest, v)
	}
	return newest
}
