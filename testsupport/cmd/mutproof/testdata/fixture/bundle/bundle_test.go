package bundle_test

import (
	"os"
	"path/filepath"
	"testing"

	"fixture/bundle"
)

// TestTheEntryIsBuilt reads the built bundle, as the UI's tests read theirs. A copy without the
// built bundle holds no entry module, so the test is red before any patch.
func TestTheEntryIsBuilt(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "ui", "browser", "dist", bundle.Entry())); err != nil {
		t.Fatal(err)
	}
}
