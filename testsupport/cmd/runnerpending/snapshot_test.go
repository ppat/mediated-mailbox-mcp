package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyReportsEveryDifference changes each kind of saved state after the snapshot and requires
// verify to name each one.
func TestVerifyReportsEveryDifference(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "content.go"), "package p\n", 0o600)
	writeFile(t, filepath.Join(root, "mode.go"), "package p\n", 0o600)
	snap, err := takeSnapshot(root, []string{"content.go", "mode.go", "created/new.go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := snap.verify(); err != nil {
		t.Fatalf("verify reported a difference before anything changed: %v", err)
	}
	writeFile(t, filepath.Join(root, "content.go"), "package q\n", 0o600)
	if err := os.Chmod(filepath.Join(root, "mode.go"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "created"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "created", "new.go"), "package created\n", 0o600)
	err = snap.verify()
	if err == nil {
		t.Fatal("verify reported no difference")
	}
	for _, want := range []string{
		"content.go differs",
		"mode.go differs",
		"new.go did not exist before the patch and still does",
		"directory " + filepath.Join(root, "created") + " did not exist",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("verify's error lacks %q:\n%v", want, err)
		}
	}
}

// TestRestoreChecksWhatItWrote replaces a saved file with a symbolic link to a copy of it. Writing the
// saved bytes back goes through the link and succeeds, so only the check after the writes can see
// that the path is not as it was.
func TestRestoreChecksWhatItWrote(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gate.go")
	writeFile(t, path, "package gate\n", 0o600)
	snap, err := takeSnapshot(root, []string{"gate.go"})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "elsewhere.go"), "package gate\n", 0o600)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere.go", path); err != nil {
		t.Fatal(err)
	}
	if err := snap.restore(); err == nil || !strings.Contains(err.Error(), "gate.go differs") {
		t.Fatalf("restore returned %v, want it to report gate.go", err)
	}
}

func TestChangedPathsNamesChangedRemovedAndNewPaths(t *testing.T) {
	before := map[string]string{"changed": "?? 1", "removed": "?? 2", "same": "?? 3"}
	after := map[string]string{"changed": "?? 9", "same": "?? 3", "new": "?? 4"}
	if got, want := changedPaths(before, after), []string{"changed", "new", "removed"}; !slices.Equal(got, want) {
		t.Errorf("changedPaths = %q, want %q", got, want)
	}
}
