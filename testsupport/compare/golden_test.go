package compare

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenDiffLocalizesAChange proves that a mismatch is reported through the shared comparison
// options as a localised diff naming the changed line, rather than as the whole file printed twice.
func TestGoldenDiffLocalizesAChange(t *testing.T) {
	want, got := mismatchFixture(t)
	d := goldenDiff(want, got)
	if d == "" {
		t.Fatal("goldenDiff reports no difference between values that differ")
	}
	if !strings.Contains(d, "TEN CHANGED") {
		t.Fatalf("the diff does not show the changed line:\n%s", d)
	}
	if !strings.Contains(d, "identical") {
		t.Fatalf("the diff prints the whole file rather than localising the change:\n%s", d)
	}
}

// mismatchFixture reads testdata/golden/mismatch.golden and returns it alongside a copy with one
// line changed.
func mismatchFixture(t *testing.T) (want, got []byte) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "golden", "mismatch.golden"))
	if err != nil {
		t.Fatal(err)
	}
	got = bytes.Replace(want, []byte("line 10\n"), []byte("line TEN CHANGED\n"), 1)
	if bytes.Equal(got, want) {
		t.Fatal("the fixture does not hold the line this test means to change, so it proves nothing")
	}
	return want, got
}

// TestGoldenScenario runs one golden scenario, chosen by GOLDEN_SCENARIO, and is otherwise a no-op.
// The tests below set it and run this test as a go test subprocess, because Golden's failure path
// calls t.Errorf or t.Fatal, and nesting that call under t.Run cannot observe it without also
// failing the outer test, since (*testing.T).Fail marks every ancestor failed unconditionally,
// exactly what a test proving a failure must not do to itself. A subprocess's exit code and output
// are what is left to observe it from outside.
func TestGoldenScenario(t *testing.T) {
	switch scenario := os.Getenv("GOLDEN_SCENARIO"); scenario {
	case "":
		t.Skip("set GOLDEN_SCENARIO to run a scenario directly")
	case "mismatch":
		_, got := mismatchFixture(t)
		Golden(t, "mismatch.golden", got)
	case "missing":
		Golden(t, "does-not-exist.golden", []byte("anything"))
	case "escape":
		Golden(t, "../escape.golden", []byte("anything"))
	case "escape-mid":
		Golden(t, "a/../../escape.golden", []byte("anything"))
	default:
		t.Fatalf("unknown GOLDEN_SCENARIO %q", scenario)
	}
}

// runScenario runs TestGoldenScenario as a subprocess with GOLDEN_SCENARIO set to scenario, in this
// package's own directory, and returns whether it passed and what it printed.
func runScenario(t *testing.T, scenario string) (passed bool, output string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestGoldenScenario$", "-v", ".")
	cmd.Env = append(os.Environ(), "GOLDEN_SCENARIO="+scenario)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("go test could not run: %v\n%s", err, out)
	}
	return err == nil, string(out)
}

// TestGoldenMismatchFails proves that comparing against a golden file whose content differs fails
// the test.
func TestGoldenMismatchFails(t *testing.T) {
	passed, out := runScenario(t, "mismatch")
	if passed {
		t.Fatalf("the mismatch scenario passed, want it to fail:\n%s", out)
	}
	if !strings.Contains(out, "differs from the recorded golden file") {
		t.Fatalf("the failure does not explain that the content differs:\n%s", out)
	}
}

// TestGoldenNameMustStayInsideGoldenDir proves that a name reaching outside testdata/golden, such as
// one meant to record a file elsewhere in the repository, fails rather than being followed. A
// leading ".." and one buried after an ordinary segment are both covered, because a check no
// stronger than a leading-".." prefix test would miss the second. "a/../../escape.golden" reaches
// outside testdata/golden exactly as "../escape.golden" does, yet it does not start with "..".
func TestGoldenNameMustStayInsideGoldenDir(t *testing.T) {
	for _, scenario := range []string{"escape", "escape-mid"} {
		t.Run(scenario, func(t *testing.T) {
			passed, out := runScenario(t, scenario)
			if passed {
				t.Fatalf("a traversal name passed, want it to fail:\n%s", out)
			}
			if !strings.Contains(out, "must be a plain relative path") {
				t.Fatalf("the failure does not explain that the name must stay inside testdata/golden:\n%s", out)
			}
		})
	}
}

// TestGoldenMissingFileFails proves that a golden file which does not exist fails an ordinary run,
// rather than being written silently.
func TestGoldenMissingFileFails(t *testing.T) {
	const name = "does-not-exist.golden"
	if _, err := os.Stat(filepath.Join("testdata", "golden", name)); err == nil {
		t.Fatalf("%s exists, so this test proves nothing", name)
	}

	passed, out := runScenario(t, "missing")
	if passed {
		t.Fatalf("the missing-file scenario passed, want it to fail:\n%s", out)
	}
	if !strings.Contains(out, "does not exist") {
		t.Fatalf("the failure does not explain that the golden file is missing:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join("testdata", "golden", name)); err == nil {
		t.Fatal("the scenario created the missing golden file instead of failing")
	}
}

// withUpdate runs fn with *update set to true, restoring it afterwards, so a test using it runs
// independently of the flag go test itself was invoked with.
func withUpdate(t *testing.T, fn func()) {
	t.Helper()
	prev := *update
	*update = true
	t.Cleanup(func() { *update = prev })
	fn()
}

// withoutUpdate runs fn with *update set to false, restoring it afterwards, so a test proving that a
// comparison actually happened is not silently turned into another write by -update on the
// package's own invocation.
func withoutUpdate(t *testing.T, fn func()) {
	t.Helper()
	prev := *update
	*update = false
	t.Cleanup(func() { *update = prev })
	fn()
}

// TestGoldenUpdateWritesFile proves that -update writes got to the golden file. It runs in a
// temporary directory, changed into with t.Chdir, so it never touches the checked-out tree.
func TestGoldenUpdateWritesFile(t *testing.T) {
	t.Chdir(t.TempDir())
	const name = "update-writes.golden"
	path := filepath.Join("testdata", "golden", name)

	content := []byte("written by -update\n")
	withUpdate(t, func() { Golden(t, name, content) })

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s after -update: %v", path, err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("the written file holds %q, want %q", got, content)
	}
}

// TestGoldenRoundTripIsByteExact proves that content written by -update compares equal, byte for
// byte, on the ordinary run right after, forced with -update off so the test proves a comparison
// happened rather than another write, whatever the package itself was run with. The content carries
// no trailing newline, a trailing space, and a byte outside valid UTF-8, none of which any formatter
// may touch on a golden file and none of which -update or Golden's own read path may normalise away.
// It runs in a temporary directory, changed into with t.Chdir, so it never touches the checked-out
// tree.
func TestGoldenRoundTripIsByteExact(t *testing.T) {
	t.Chdir(t.TempDir())
	const name = "round-trip.golden"

	content := []byte("no trailing newline, trailing space \xff\x00")
	withUpdate(t, func() { Golden(t, name, content) })
	withoutUpdate(t, func() { Golden(t, name, content) })
}
