package migrations_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
)

// newestInDirectory reads the newest migration's version from this directory's file names, apart from
// the code under test.
func newestInDirectory(t *testing.T) int64 {
	t.Helper()
	paths, err := filepath.Glob("*.sql")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	var newest int64
	for _, path := range paths {
		var v int64
		if _, err := fmt.Sscanf(path, "%d_", &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		newest = max(newest, v)
	}
	return newest
}

// The chain's version is the newest migration it carries, the version a binary built with it compares
// with the schema's (ADR-0140).
func TestTheChainsVersionIsItsNewestMigration(t *testing.T) {
	if got, want := migrations.Version(), newestInDirectory(t); got != want {
		t.Errorf("Version() = %d, want %d, the newest migration in the directory", got, want)
	}
}

// A binary refuses a schema whose compatibility floor is above its chain, and one that has not applied
// its chain's newest migration, and runs on every other, a newer schema whose floor its chain meets
// among them, which is a rollback across migrations that left every chain the floor admits working (ADR-0140).
func TestABinaryRefusesASchemaItsChainCannotRunOn(t *testing.T) {
	k := newestInDirectory(t)
	for _, c := range []struct {
		name           string
		applied, floor int64
		want           error
	}{
		{"the schema its chain built", k, 0, nil},
		{"the floor at its chain", k, k, nil},
		{"a newer schema whose floor its chain meets", k + 3, k, nil},
		{"the floor one above its chain", k, k + 1, migrations.ErrBelowFloor},
		{"a newer schema whose floor is above its chain", k + 3, k + 2, migrations.ErrBelowFloor},
		{"the chain's newest migration not applied", k - 1, 0, migrations.ErrNotMigrated},
		{"nothing applied", 0, 0, migrations.ErrNotMigrated},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := migrations.Refuse(c.applied, c.floor)
			if !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
				t.Errorf("Refuse(%d, %d) with the chain at %d = %v, want %v", c.applied, c.floor, k, err, c.want)
			}
		})
	}
}
