package compare

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// update rewrites every golden file a test compares against, in place of comparing to it. It is
// registered here, once, so every package's tests share one flag rather than each registering its
// own and panicking on the second registration. The gating CI run passes no flag to go test, so it
// stays false there and a run never writes a golden file by itself.
var update = flag.Bool("update", false, "rewrite golden files instead of comparing against them")

// Golden compares got against the golden file at testdata/golden/name, read relative to the calling
// test's own package directory, the way CLAUDE.md's testdata convention reads every testdata file
// (ADR-0070). Run with -update, it writes got to that file instead of comparing, which is the
// deliberate way a golden file is recorded or re-recorded. got is written and read back exactly, so
// running -update once and then an ordinary run right after leaves nothing to compare unequal.
//
// name must be a plain relative path inside testdata/golden, not absolute and not containing "..",
// so it cannot name a file outside that directory.
//
// A golden file that does not exist fails the test rather than being created silently, so a
// misspelled name, or a test nobody has yet run with -update, is caught rather than starting a new
// file's history unnoticed.
func Golden(t *testing.T, name string, got []byte) {
	t.Helper()
	if !filepath.IsLocal(name) {
		t.Fatalf("golden file name %q must be a plain relative path inside testdata/golden, not absolute and not containing \"..\"", name)
	}
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s does not exist. Run go test -update in this package to record it", path)
	}
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if d := goldenDiff(want, got); d != "" {
		t.Errorf("%s differs from the recorded golden file (-want +got):\n%s", path, d)
	}
}

// goldenDiff reports how want and got differ, through the shared comparison options, or the empty
// string when they match exactly. It compares as strings rather than as byte slices so a multi-line
// difference is localised rather than printed as a byte-by-byte index walk.
func goldenDiff(want, got []byte) string {
	return cmp.Diff(string(want), string(got), Options)
}
