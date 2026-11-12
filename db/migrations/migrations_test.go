package migrations_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// TestTheEmbeddedChainIsTheDirectory requires the chain the binary carries to be every SQL file of
// this directory, byte for byte, and nothing else, at the top of the file system goose reads, so the
// migrator and the integration runs apply the chain that is checked in (ADR-0139).
func TestTheEmbeddedChainIsTheDirectory(t *testing.T) {
	want := map[string]string{}
	paths, err := filepath.Glob("*.sql")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	for _, path := range paths {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want[path] = string(text)
	}
	got := map[string]string{}
	err = fs.WalkDir(migrations.Chain(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := fs.ReadFile(migrations.Chain(), path)
		got[path] = string(text)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the embedded chain (-directory +embedded):\n%s", diff)
	}
}
