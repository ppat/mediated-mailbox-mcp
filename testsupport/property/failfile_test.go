package property

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// No fail file of rapid's own is kept anywhere in the repository.
func TestNoRapidFailFileIsKept(t *testing.T) {
	root := filepath.Join("..", "..")
	// The search covers the whole module only if it starts where go.mod is.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("the search does not start at the module root: %v", err)
	}
	found, err := rapidFailFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("%s is one of rapid's own fail files. Delete it, and keep the failing case in the failing-case store instead", f)
	}
}

func TestRapidFailFilesAreFound(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "core", "x", "testdata", "rapid", "TestX", "TestX-1.fail")
	if err := os.MkdirAll(filepath.Dir(stale), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := rapidFailFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{stale}, found, compare.Options); diff != "" {
		t.Errorf("fail files found (-want +got):\n%s", diff)
	}
}
