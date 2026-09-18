package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The tests below run the runner over a copy of testdata/fixture in a temporary directory, so a
// killed test run can never leave this checkout patched. Every demonstration runs real go test and
// git apply commands.

// fixtureRoot copies the fixture module into a temporary directory and gives it a go.mod requiring
// the same rapid version as the repository's, so the fixture builds from the module cache the
// repository already fills.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "fixture"))); err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join("..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	gosum, err := os.ReadFile(filepath.Join("..", "..", "..", "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	goVersion := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(gomod)
	rapid := regexp.MustCompile(`(?m)^\s*pgregory\.net/rapid (\S+)`).FindSubmatch(gomod)
	if goVersion == nil || rapid == nil {
		t.Fatal("the repository's go.mod names no go version or no rapid version")
	}
	mod := "module fixture\n\ngo " + string(goVersion[1]) + "\n\nrequire pgregory.net/rapid " + string(rapid[1]) + "\n"
	var sum strings.Builder
	for line := range strings.Lines(string(gosum)) {
		if strings.HasPrefix(line, "pgregory.net/rapid "+string(rapid[1])) {
			sum.WriteString(line)
		}
	}
	//nolint:gosec // The path is the go.mod of the test's own temporary directory.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte(sum.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// The file leftover.patch leaves behind is ignored, as build output is, so the working tree check
	// does not see it and only the tests run after the restore can.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("leftover\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// fixtureEnv is this process's environment without any rapid setting, GOFLAGS or go env file, plus
// the given settings, so a gating run's or a developer's settings do not reach the fixture.
func fixtureEnv(settings ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RAPID_") && !strings.HasPrefix(kv, "GOFLAGS=") && !strings.HasPrefix(kv, "GOENV=") {
			env = append(env, kv)
		}
	}
	// GOENV=off keeps a developer's go env file, which can set GOFLAGS, away from the fixture. A setting
	// given here comes later and wins.
	return append(append(env, "GOENV=off"), settings...)
}

func patchPath(root, pkg, name string) string {
	return filepath.Join(root, pkg, "testdata", "mutations", name+".patch")
}

// requireUnpatched fails unless the file at rel under root holds the fixture's original bytes.
func requireUnpatched(t *testing.T, root, rel string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "fixture", rel))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(want), string(got), compare.Options); diff != "" {
		t.Errorf("%s was not restored (-want +got):\n%s", rel, diff)
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s exists, and must not: %v", path, err)
	}
}

// outcome is what a test asserts about a result, in values it can compare.
type outcome struct {
	Red         []string
	Broken      []string
	StayedGreen []string
	Surviving   bool
	NotRestored bool
	Held        bool
}

func outcomeOf(r result) outcome {
	o := outcome{
		StayedGreen: r.stayedGreen,
		Surviving:   r.surviving(),
		NotRestored: r.notRestored != "",
		Held:        r.held(),
	}
	for _, id := range r.red {
		o.Red = append(o.Red, id.String())
	}
	for pkg := range r.broken {
		o.Broken = append(o.Broken, pkg)
	}
	return o
}

func demonstrateFixture(t *testing.T, root, pkg, patch string, env []string) (result, error) {
	t.Helper()
	return demonstrate(t.Context(), root, env, patchPath(root, pkg, patch))
}

func TestRemovedMechanismTurnsTheRequiredTestRed(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "red", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	want := outcome{Red: []string{"fixture/gate TestRefusesFlagged"}, Held: true}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
	rows := []string{"| The gate refuses flagged items | Allow returns true without reading the flag | `TestRefusesFlagged` in `fixture/gate` | " +
		time.Now().Format("2006-01-02") + " · EVIDENCE |"}
	got, _ := ledgerRows([]ledgerEntry{{res.preamble.control, &res}})
	if diff := cmp.Diff(rows, got, compare.Options); diff != "" {
		t.Errorf("ledger rows (-want +got):\n%s", diff)
	}
	requireUnpatched(t, root, "gate/gate.go")
}

func TestPatchLeavingEveryTestGreenIsASurvivingMutant(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "survive", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	want := outcome{StayedGreen: []string{"TestRefusesFlagged"}, Surviving: true}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
	requireUnpatched(t, root, "gate/gate.go")
}

func TestRequiredTestStayingGreenFailsWhileAnotherGoesRed(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "wrong", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	want := outcome{Red: []string{"fixture/gate TestAllowsClean"}, StayedGreen: []string{"TestRefusesFlagged"}}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
}

// TestRequiredTestMustGoRedInEveryPackage names a test that exists in two packages, of which the
// patch reaches only one.
func TestRequiredTestMustGoRedInEveryPackage(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "twin", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	want := outcome{Red: []string{"fixture/gate TestRefusesFlagged"}, StayedGreen: []string{"TestRefusesFlagged"}}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
}

func TestRequiredTestThatDoesNotExistIsRefused(t *testing.T) {
	root := fixtureRoot(t)
	_, err := demonstrateFixture(t, root, "gate", "missing", fixtureEnv())
	if err == nil || !strings.Contains(err.Error(), "did not run and pass before the patch was applied: TestDoesNotExist") {
		t.Fatalf("got error %v, want the missing test named before the patch is applied", err)
	}
}

func TestRedBeforeThePatchIsRefused(t *testing.T) {
	root := fixtureRoot(t)
	_, err := demonstrateFixture(t, root, "baseline", "double", fixtureEnv())
	if err == nil || !strings.Contains(err.Error(), "red before the patch is applied") || !strings.Contains(err.Error(), "TestUnrelatedFailure") {
		t.Fatalf("got error %v, want a refusal because the tests are red unpatched", err)
	}
	requireUnpatched(t, root, "baseline/baseline.go")
}

func TestPatchBreakingTheBuildIsNotADemonstration(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "build", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	want := outcome{Broken: []string{"fixture/gate"}, StayedGreen: []string{"TestRefusesFlagged"}}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
	requireUnpatched(t, root, "gate/gate.go")
}

func TestTestsRedAfterRestoringFailTheDemonstration(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "leftover", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	if res.held() || !strings.Contains(res.notRestored, "TestNoLeftover") {
		t.Fatalf("held %v with notRestored %q, want a failed demonstration naming TestNoLeftover", res.held(), res.notRestored)
	}
}

// TestScheduledCountReachesRapid uses a patch that breaks the mechanism only from the 500th
// generated case on. rapid's default count is 100, so the property goes red only if the scheduled
// count reaches the test process.
func TestScheduledCountReachesRapid(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		root := fixtureRoot(t)
		res, err := demonstrateFixture(t, root, "count", "rare", fixtureEnv("RAPID_CHECKS=100", "RAPID_SCHEDULED_CHECKS=1000"))
		if err != nil {
			t.Fatal(err)
		}
		want := outcome{Red: []string{"fixture/count TestHoldsForEveryCase"}, Held: true}
		if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
			t.Errorf("outcome (-want +got):\n%s", diff)
		}
		// The red property would have written a fail file here without RAPID_NOFAILFILE=true.
		requireAbsent(t, filepath.Join(root, "count", "testdata", "rapid"))
	})
	t.Run("unset", func(t *testing.T) {
		root := fixtureRoot(t)
		_, err := demonstrateFixture(t, root, "count", "rare", fixtureEnv("RAPID_CHECKS=100"))
		if err == nil || !strings.Contains(err.Error(), "RAPID_SCHEDULED_CHECKS must be a positive number") {
			t.Fatalf("got error %v, want a refusal naming RAPID_SCHEDULED_CHECKS", err)
		}
	})
}

// TestInterruptRestoresThePatchedFiles cancels the run while a patched test blocks, as an interrupt
// does. It requires every file back as it was and the blocked test process gone. The patched code
// writes its process ID to a file named blocked before it blocks.
func TestInterruptRestoresThePatchedFiles(t *testing.T) {
	root := fixtureRoot(t)
	blocked := filepath.Join(root, "gate", "blocked")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pid := make(chan int, 1)
	go func() {
		for ctx.Err() == nil {
			if data, err := os.ReadFile(blocked); err == nil && len(data) > 0 {
				n, err := strconv.Atoi(string(data))
				if err == nil {
					pid <- n
				}
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	_, err := demonstrate(ctx, root, fixtureEnv(), patchPath(root, "gate", "interrupt"))
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want the interruption", err)
	}
	var stop *stopError
	if errors.As(err, &stop) {
		t.Fatalf("restoring failed: %v", err)
	}
	requireUnpatched(t, root, "gate/gate.go")
	requireAbsent(t, filepath.Join(root, "gate", "newdir"))
	if !strings.Contains(err.Error(), "the working tree differs from before the patch at gate/blocked") {
		t.Errorf("the error does not name the file the patched code wrote:\n%v", err)
	}
	select {
	case n := <-pid:
		requireExited(t, n)
	default:
		t.Fatal("the patched test never started, so the run was not interrupted while it blocked")
	}
}

// requireExited waits briefly for the process to be gone, counting a process that has exited but
// not yet been reaped as gone.
func requireExited(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if err != nil {
			return
		}
		// The state is the field after the parenthesised command name.
		if _, rest, ok := strings.Cut(string(stat), ") "); ok && strings.HasPrefix(rest, "Z") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the blocked test process %d is still running after the interrupt", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestFailedRestoreStopsTheRun uses a patch whose tests write into a directory the patch adds, so the
// directory cannot be removed. The rest must still be restored, and no later patch may run.
func TestFailedRestoreStopsTheRun(t *testing.T) {
	root := fixtureRoot(t)
	var out bytes.Buffer
	ok := runAll(t.Context(), &out, root, fixtureEnv(), []string{patchPath(root, "gate", "strand"), patchPath(root, "count", "rare")})
	if ok {
		t.Error("runAll reported success")
	}
	if !strings.Contains(out.String(), "restoring the patched files failed") {
		t.Errorf("output does not report the failed restore:\n%s", out.String())
	}
	if strings.Contains(out.String(), "rare.patch") {
		t.Errorf("a patch ran after the failed restore:\n%s", out.String())
	}
	// The patch that never ran still leaves its control without a row.
	if want := `No ledger row for "The mechanism holds for every case"`; !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, out.String())
	}
	requireUnpatched(t, root, "gate/gate.go")
}

func TestRunAllReportsEachPatch(t *testing.T) {
	root := fixtureRoot(t)
	var out bytes.Buffer
	ok := runAll(t.Context(), &out, root, fixtureEnv(), []string{patchPath(root, "gate", "red"), patchPath(root, "gate", "survive")})
	if ok {
		t.Error("runAll reported success with a surviving mutant among the patches")
	}
	for _, want := range []string{
		"PASS " + patchPath(root, "gate", "red"),
		"FAIL " + patchPath(root, "gate", "survive"),
		"surviving mutant",
		"Ledger rows for docs/MUTATIONS.md\n| The gate refuses flagged items | (1) Allow returns true without reading the flag<br>(2) Allow is rewritten to the same logic |",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

// TestGOFLAGSIsRefused sets go test flags through GOFLAGS, in the environment and in the file go
// env -w writes. Unrefused, -run hides the test that is red before the patch, and -short makes rapid
// run a fifth of the scheduled cases, so the removal of the mechanism goes unseen.
func TestGOFLAGSIsRefused(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "goenv")
	writeFile(t, envFile, "GOFLAGS=-short\n", 0o600)
	cases := []struct {
		name, pkg, patch string
		env              []string
	}{
		{"-run", "baseline", "double", fixtureEnv("GOFLAGS=-run=TestDoubleZero")},
		{"-short", "count", "rare", fixtureEnv("RAPID_SCHEDULED_CHECKS=1000", "GOFLAGS=-count=1 -short")},
		{"go env -w", "count", "rare", fixtureEnv("RAPID_SCHEDULED_CHECKS=1000", "GOENV="+envFile)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := fixtureRoot(t)
			_, err := demonstrateFixture(t, root, c.pkg, c.patch, c.env)
			if err == nil || !strings.Contains(err.Error(), "GOFLAGS is") {
				t.Fatalf("got error %v, want a refusal naming GOFLAGS", err)
			}
		})
	}
}

// TestEmptiedDirectoryIsRestored uses patches that delete, or move elsewhere, the only file in
// gate/limit, which takes the directory with it.
func TestEmptiedDirectoryIsRestored(t *testing.T) {
	for _, patch := range []string{"delete", "moveout"} {
		t.Run(patch, func(t *testing.T) {
			root := fixtureRoot(t)
			res, err := demonstrateFixture(t, root, "gate", patch, fixtureEnv())
			if err != nil {
				t.Fatal(err)
			}
			if !res.held() {
				t.Errorf("the demonstration did not hold:\n%s", res.report())
			}
			requireUnpatched(t, root, "gate/limit/limit.go")
			requireAbsent(t, filepath.Join(root, "limits"))
		})
	}
}

// TestRenameIsRestored uses a patch that also renames a file, which deletes the file's old path.
func TestRenameIsRestored(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "gate", "rename", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	if !res.held() {
		t.Errorf("the demonstration did not hold:\n%s", res.report())
	}
	requireUnpatched(t, root, "twin/twin.go")
	requireAbsent(t, filepath.Join(root, "twin", "renamed.go"))
}

// TestModeChangeIsRestored uses a patch that also makes the file it changes executable.
func TestModeChangeIsRestored(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "gate", "gate.go")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := demonstrateFixture(t, root, "gate", "mode", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	if !res.held() {
		t.Errorf("the demonstration did not hold:\n%s", res.report())
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode() != before.Mode() {
		t.Errorf("gate.go has mode %s after the run, and had %s before", after.Mode(), before.Mode())
	}
}

func TestPatchTouchingTestCodeIsRefused(t *testing.T) {
	cases := map[string]string{
		"testonly": "gate/gate_test.go",
		"testdata": "gate/testdata/input.txt",
	}
	for patch, path := range cases {
		t.Run(patch, func(t *testing.T) {
			root := fixtureRoot(t)
			_, err := demonstrateFixture(t, root, "gate", patch, fixtureEnv())
			if err == nil || !strings.Contains(err.Error(), "the patch touches test code") || !strings.Contains(err.Error(), path) {
				t.Fatalf("got error %v, want a refusal naming %s", err, path)
			}
		})
	}
}

func TestSkippedRequiredTestIsRefused(t *testing.T) {
	root := fixtureRoot(t)
	_, err := demonstrateFixture(t, root, "gate", "skip", fixtureEnv())
	if err == nil || !strings.Contains(err.Error(), "did not run and pass before the patch was applied: TestSkipped") {
		t.Fatalf("got error %v, want the skipped test named", err)
	}
}

// TestStrayFileFailsTheDemonstrationAndStopsTheRun uses a patch whose code writes a file into another
// package, outside every path the patch names. No test notices it, so only the working tree check can.
func TestStrayFileFailsTheDemonstrationAndStopsTheRun(t *testing.T) {
	root := fixtureRoot(t)
	var out bytes.Buffer
	ok := runAll(t.Context(), &out, root, fixtureEnv(), []string{patchPath(root, "gate", "stray"), patchPath(root, "gate", "red")})
	if ok {
		t.Error("runAll reported success")
	}
	if !strings.Contains(out.String(), "the working tree differs from before the patch at twin/stray.go") {
		t.Errorf("output does not name the stray file:\n%s", out.String())
	}
	if strings.Contains(out.String(), "red.patch") {
		t.Errorf("a patch ran after the working tree changed:\n%s", out.String())
	}
}

// TestSeedIsChosenAndRecorded runs a patch with no RAPID_SEED in the environment. The fixture's
// TestSeedIsSet fails unless the runs have one, and the report must record it.
func TestSeedIsChosenAndRecorded(t *testing.T) {
	root := fixtureRoot(t)
	res, err := demonstrateFixture(t, root, "count", "rare", fixtureEnv("RAPID_SCHEDULED_CHECKS=1000"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := strconv.ParseUint(res.seed, 10, 64); err != nil || n == 0 {
		t.Fatalf("the result records seed %q", res.seed)
	}
	if want := "every run had RAPID_SEED=" + res.seed + " and RAPID_CHECKS 1000"; !strings.Contains(res.report(), want) {
		t.Errorf("report lacks %q:\n%s", want, res.report())
	}
}

func TestSeedFromTheEnvironmentIsKept(t *testing.T) {
	_, seed, checks, err := testEnv(fixtureEnv("RAPID_SEED=77", "RAPID_CHECKS=300"), false)
	if err != nil {
		t.Fatal(err)
	}
	if seed != "77" || checks != "300" {
		t.Errorf("testEnv kept seed %q and checks %q, want 77 and 300", seed, checks)
	}
}

func TestLedgerRowsOnePerCompleteControl(t *testing.T) {
	date := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	entry := func(r result) ledgerEntry {
		r.date = date
		return ledgerEntry{r.preamble.control, &r}
	}
	held := func(control, removes string, red ...testID) ledgerEntry {
		return entry(result{preamble: preamble{control: control, removes: removes}, red: red})
	}
	entries := []ledgerEntry{
		held("Gate | refuses", "Allow returns true", testID{"m/gate", "TestA"}, testID{"m/gate", "TestB"}, testID{"m/other", "TestC"}),
		held("Redaction", "Redact returns its input", testID{"m/redact", "TestR"}),
		held("Gate | refuses", "Allow ignores the flag"),
		// A patch that held beside one where a named test stayed green.
		held("Scanner", "Scan finds nothing", testID{"m/scan", "TestS"}),
		entry(result{preamble: preamble{control: "Scanner", removes: "Scan stops early"}, red: []testID{{"m/scan", "TestT"}}, stayedGreen: []string{"TestS"}}),
		// A patch that held beside one that could not be judged or never ran.
		held("Classifier", "Classify returns clean", testID{"m/classify", "TestK"}),
		{control: "Classifier"},
		// A patch whose preamble could not be read.
		{},
	}
	wantRows := []string{
		"| Gate \\| refuses | (1) Allow returns true<br>(2) Allow ignores the flag | (1) `TestA`, `TestB` in `m/gate`, `TestC` in `m/other`<br>(2) none, a surviving mutant | 2026-09-18 · EVIDENCE · open, a surviving mutant |",
		"| Redaction | Redact returns its input | `TestR` in `m/redact` | 2026-09-18 · EVIDENCE |",
	}
	rows, incomplete := ledgerRows(entries)
	if diff := cmp.Diff(wantRows, rows, compare.Options); diff != "" {
		t.Errorf("ledger rows (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"Scanner", "Classifier"}, incomplete, compare.Options); diff != "" {
		t.Errorf("incomplete controls (-want +got):\n%s", diff)
	}
}

// TestRunAllPrintsNoRowForAFailedRemoval runs a patch that holds and one of the same control whose
// named test stays green.
func TestRunAllPrintsNoRowForAFailedRemoval(t *testing.T) {
	root := fixtureRoot(t)
	var out bytes.Buffer
	runAll(t.Context(), &out, root, fixtureEnv(), []string{patchPath(root, "gate", "red"), patchPath(root, "gate", "wrong")})
	if strings.Contains(out.String(), "Ledger rows") {
		t.Errorf("a row was printed for an incomplete control:\n%s", out.String())
	}
	if want := `No ledger row for "The gate refuses flagged items", because its demonstration is incomplete`; !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, out.String())
	}
}

// TestHangupRestoresThePatchedFiles runs the built program and sends it SIGHUP, as closing its
// terminal does, while a patched test blocks.
func TestHangupRestoresThePatchedFiles(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "runner")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	root := fixtureRoot(t)
	cmd := exec.Command(bin, filepath.Join("gate", "testdata", "mutations", "interrupt.patch"))
	cmd.Dir = root
	cmd.Env = fixtureEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "gate", "blocked")
	deadline := time.Now().Add(60 * time.Second)
	var pid int
	for {
		if data, err := os.ReadFile(blocked); err == nil && len(data) > 0 {
			if pid, err = strconv.Atoi(string(data)); err == nil {
				break
			}
		}
		if time.Now().After(deadline) {
			if err := cmd.Process.Kill(); err != nil {
				t.Log(err)
			}
			t.Fatalf("the patched test never started:\n%s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	// The blocked test runs in its own process group, which survives the runner if the runner dies.
	t.Cleanup(func() {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Log(err)
		}
	})
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Errorf("the runner ended with %v, want exit status 1 after restoring:\n%s", err, out.String())
	}
	requireUnpatched(t, root, "gate/gate.go")
	requireAbsent(t, filepath.Join(root, "gate", "newdir"))
	if !strings.Contains(out.String(), "the working tree differs from before the patch at gate/blocked") {
		t.Errorf("the output does not name the file the patched code wrote:\n%s", out.String())
	}
	requireExited(t, pid)
}

// TestUncheckedTreeIsSaid interrupts a run after moving the fixture's .git directory away, so the
// comparison of the working tree after the restore cannot run. The error must say so.
func TestUncheckedTreeIsSaid(t *testing.T) {
	root := fixtureRoot(t)
	blocked := filepath.Join(root, "gate", "blocked")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		for ctx.Err() == nil {
			if data, err := os.ReadFile(blocked); err == nil && len(data) > 0 {
				if err := os.Rename(filepath.Join(root, ".git"), filepath.Join(root, "git-moved")); err != nil {
					t.Error(err)
				}
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	_, err := demonstrate(ctx, root, fixtureEnv(), patchPath(root, "gate", "interrupt"))
	if err == nil || !strings.Contains(err.Error(), "the working tree was not checked against its state before the patch") {
		t.Fatalf("got error %v, want it to say the working tree was not checked", err)
	}
	requireUnpatched(t, root, "gate/gate.go")
}
