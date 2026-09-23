package property_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The tests below run go test over a copy of testdata/fixture, a module that replaces this
// repository's module with this checkout, so the store's writes land in a temporary directory and
// every run uses exactly the settings it names.

const repoRoot = "../.."

// fixture copies testdata/fixture into a temporary module and returns its directory.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("testdata", "fixture"))); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goVersion := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(gomod)
	rapid := regexp.MustCompile(`(?m)^\s*pgregory\.net/rapid (\S+)`).FindSubmatch(gomod)
	if goVersion == nil || rapid == nil {
		t.Fatal("the repository's go.mod names no go version or no rapid version")
	}
	mod := "module fixture\n\ngo " + string(goVersion[1]) + "\n\n" +
		"require (\n\tgithub.com/ppat/mediated-mailbox-mcp v0.0.0\n\tpgregory.net/rapid " + string(rapid[1]) + "\n)\n\n" +
		"replace github.com/ppat/mediated-mailbox-mcp => " + root + "\n"
	gosum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // The path is the go.mod of the test's own temporary directory.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // The path is the go.sum of the test's own temporary directory.
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), gosum, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// gating is the setting set of a gating run.
var gating = []string{"RAPID_SEED=1", "RAPID_CHECKS=100", "RAPID_NOFAILFILE=true"}

// run runs one fixture test with this process's environment stripped of every rapid setting and
// of anything that could change how go test runs, plus the given settings. It returns whether the
// run passed and its output.
func run(t *testing.T, dir, pkg, test string, settings ...string) (bool, string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-run", "^"+test+"$", "./"+pkg)
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RAPID_") && !strings.HasPrefix(kv, "GOFLAGS=") && !strings.HasPrefix(kv, "GOENV=") && !strings.HasPrefix(kv, "FIXTURE_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "GOENV=off", "GOFLAGS=-mod=mod")
	cmd.Env = append(cmd.Env, settings...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("go test could not run: %v\n%s", err, out)
	}
	return err == nil, string(out)
}

func requireOutcome(t *testing.T, passed bool, out string, wantPass bool, wantText string) {
	t.Helper()
	if passed != wantPass {
		t.Fatalf("the run passed=%v, want passed=%v\n%s", passed, wantPass, out)
	}
	if wantText != "" && !strings.Contains(out, wantText) {
		t.Fatalf("the run's output lacks %q\n%s", wantText, out)
	}
}

// A run that is not the fixed, repeatable gating run fails before generating anything.
func TestARunWithoutTheGatingSettingsFails(t *testing.T) {
	dir := fixture(t)
	passed, out := run(t, dir, "report", "TestMix", gating...)
	requireOutcome(t, passed, out, true, "")
	cases := []struct {
		name     string
		settings []string
		want     string
	}{
		{"seed unset", []string{"RAPID_CHECKS=100", "RAPID_NOFAILFILE=true"}, "RAPID_SEED must be set to a non-zero number"},
		{"seed zero", []string{"RAPID_SEED=0", "RAPID_CHECKS=100", "RAPID_NOFAILFILE=true"}, "RAPID_SEED must be set to a non-zero number"},
		{"no fail file unset", []string{"RAPID_SEED=1", "RAPID_CHECKS=100"}, "RAPID_NOFAILFILE must be true"},
		{"no fail file false", []string{"RAPID_SEED=1", "RAPID_CHECKS=100", "RAPID_NOFAILFILE=false"}, "RAPID_NOFAILFILE must be true"},
	}
	// Report runs through the report fixture and Check through the store fixture, whose property fails
	// under the gating settings. So the message, not the red alone, shows the settings check fired.
	for _, fx := range []struct{ pkg, test string }{{"report", "TestMix"}, {"store", "TestPlantedFault"}} {
		for _, c := range cases {
			t.Run(fx.pkg+"/"+c.name, func(t *testing.T) {
				passed, out := run(t, dir, fx.pkg, fx.test, c.settings...)
				requireOutcome(t, passed, out, false, c.want)
			})
		}
	}
}

// A generator narrowed so a kind of input its stated mix requires is never produced fails the
// report, although nothing the property tests is wrong and no draw is refused.
func TestTheReportFailsAGeneratorThatMissesItsMix(t *testing.T) {
	dir := fixture(t)
	passed, out := run(t, dir, "report", "TestMix", gating...)
	requireOutcome(t, passed, out, true, "")
	passed, out = run(t, dir, "report", "TestMix", append([]string{"FIXTURE_VARIANT=narrow"}, gating...)...)
	requireOutcome(t, passed, out, false, "the generator missed its stated mix: high at 0.0%")
}

// A found failure is stored as its arguments, still fails the run after the draw function is edited
// and the budget is too small to find it again, and a change in the arguments' shape fails loudly.
func TestAStoredFailingCaseOutlivesAGeneratorEdit(t *testing.T) {
	dir := fixture(t)
	storePath := filepath.Join(dir, "store", "testdata", "property", "TestPlantedFault.json")

	passed, out := run(t, dir, "store", "TestPlantedFault", gating...)
	requireOutcome(t, passed, out, false, "stored the failing case")
	src, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("no store written: %v", err)
	}
	var kept struct {
		Shape string
		Cases []struct{ N int }
	}
	if err := json.Unmarshal(src, &kept); err != nil {
		t.Fatal(err)
	}
	want := struct {
		Shape string
		Cases []struct{ N int }
	}{Shape: "struct{N int}", Cases: []struct{ N int }{{N: 500}}}
	if diff := cmp.Diff(want, kept, compare.Options); diff != "" {
		t.Fatalf("stored cases (-want +got):\n%s", diff)
	}

	edited := []string{"FIXTURE_VARIANT=extradraw", "RAPID_SEED=1", "RAPID_CHECKS=0", "RAPID_NOFAILFILE=true"}
	passed, out = run(t, dir, "store", "TestPlantedFault", edited...)
	requireOutcome(t, passed, out, false, "N 500 reaches the planted fault")

	// Without the store the same run passes, so only the store turned it red.
	if err := os.Rename(storePath, storePath+".aside"); err != nil {
		t.Fatal(err)
	}
	passed, out = run(t, dir, "store", "TestPlantedFault", edited...)
	requireOutcome(t, passed, out, true, "")
	if err := os.Rename(storePath+".aside", storePath); err != nil {
		t.Fatal(err)
	}

	passed, out = run(t, dir, "store", "TestPlantedFault", append([]string{"FIXTURE_VARIANT=reshaped"}, gating...)...)
	requireOutcome(t, passed, out, false, "Write each stored case out as an example-based test against the new arguments")

	if _, err := os.Stat(filepath.Join(dir, "store", "testdata", "rapid")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("rapid wrote its own fail file under testdata/rapid: %v", err)
	}
}

// A test that fails for a reason other than its property stores nothing, because no drawn case
// fails the property.
func TestAnUnrelatedFailureStoresNothing(t *testing.T) {
	dir := fixture(t)
	passed, out := run(t, dir, "store", "TestPlantedFault", append([]string{"FIXTURE_VARIANT=unrelated"}, gating...)...)
	requireOutcome(t, passed, out, false, "a failure unrelated to the property")
	if _, err := os.Stat(filepath.Join(dir, "store", "testdata", "property")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a case that never failed the property was stored: %v\n%s", err, out)
	}
}
