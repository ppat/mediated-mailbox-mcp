package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
	Held        bool
}

func outcomeOf(r result) outcome {
	o := outcome{
		StayedGreen: r.stayedGreen,
		Surviving:   r.surviving(),
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
	})
	t.Run("unset", func(t *testing.T) {
		root := fixtureRoot(t)
		_, err := demonstrateFixture(t, root, "count", "rare", fixtureEnv("RAPID_CHECKS=100"))
		if err == nil || !strings.Contains(err.Error(), "RAPID_SCHEDULED_CHECKS must be a positive number") {
			t.Fatalf("got error %v, want a refusal naming RAPID_SCHEDULED_CHECKS", err)
		}
	})
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
		"  surviving mutant: every test stayed green",
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

func TestPatchTouchingTestCodeIsRefused(t *testing.T) {
	cases := map[string]string{
		"testonly": "gate/gate_test.go",
		"testdata": "gate/testdata/input.txt",
		// git apply names only the new path of a rename, so the test file renamed away is seen through
		// the patch's rename from line.
		"testrename": "gate/gate_test.go",
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
	if want := "both runs had RAPID_SEED=" + res.seed + " and RAPID_CHECKS 1000"; !strings.Contains(res.report(), want) {
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

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// treeDigest maps every file and directory under root, .git included, to its mode and, for a file,
// a hash of its content, so two digests differ when anything in the checkout changed.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digest := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		state := info.Mode().String()
		if info.Mode().IsRegular() {
			//nolint:gosec // The walk is over the test's own temporary checkout.
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state += fmt.Sprintf(" %x", sha256.Sum256(data))
		}
		digest[path] = state
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func requireEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("%s is left in the temporary directory", e.Name())
	}
}

// TestCheckoutIsUnchanged runs every fixture patch that ends on its own, the one whose code writes a
// file into another package included, and requires the checkout byte for byte as it was and no copy
// left behind.
func TestCheckoutIsUnchanged(t *testing.T) {
	root := fixtureRoot(t)
	var patches []string
	for _, p := range []struct{ pkg, name string }{
		{"gate", "red"},
		{"gate", "survive"},
		{"gate", "wrong"},
		{"gate", "twin"},
		{"gate", "missing"},
		{"gate", "build"},
		{"gate", "skip"},
		{"gate", "stray"},
		{"gate", "testonly"},
		{"gate", "testdata"},
		{"baseline", "double"},
		{"count", "rare"},
	} {
		patches = append(patches, patchPath(root, p.pkg, p.name))
	}
	before := treeDigest(t, root)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	var out bytes.Buffer
	runAll(t.Context(), &out, root, fixtureEnv("RAPID_SCHEDULED_CHECKS=1000"), patches)
	if !strings.Contains(out.String(), "PASS "+patchPath(root, "gate", "stray")) {
		t.Errorf("the patch writing into another package did not hold:\n%s", out.String())
	}
	if diff := cmp.Diff(before, treeDigest(t, root), compare.Options); diff != "" {
		t.Errorf("the checkout changed (-before +after):\n%s", diff)
	}
	requireEmpty(t, tmp)
}

// TestInterruptLeavesTheCheckoutUnchanged cancels the run while a patched test blocks, as an
// interrupt does. The patched code writes its process ID to $BLOCKED_FILE before it blocks. The
// checkout must be as it was, the blocked test gone, no copy left, and the patch after it never run.
func TestInterruptLeavesTheCheckoutUnchanged(t *testing.T) {
	root := fixtureRoot(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	before := treeDigest(t, root)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pid := make(chan int, 1)
	go func() {
		for ctx.Err() == nil {
			if data, err := os.ReadFile(blocked); err == nil && len(data) > 0 {
				if n, err := strconv.Atoi(string(data)); err == nil {
					pid <- n
				}
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	var out bytes.Buffer
	ok := runAll(ctx, &out, root, fixtureEnv("BLOCKED_FILE="+blocked), []string{patchPath(root, "gate", "interrupt"), patchPath(root, "count", "rare")})
	if ok || !strings.Contains(out.String(), "context canceled") {
		t.Errorf("runAll returned %v, want a failure from the interruption:\n%s", ok, out.String())
	}
	if strings.Contains(out.String(), "rare.patch") {
		t.Errorf("a patch ran after the interruption:\n%s", out.String())
	}
	if want := `No ledger row for "The mechanism holds for every case"`; !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, out.String())
	}
	if diff := cmp.Diff(before, treeDigest(t, root), compare.Options); diff != "" {
		t.Errorf("the checkout changed (-before +after):\n%s", diff)
	}
	requireEmpty(t, tmp)
	select {
	case n := <-pid:
		requireExited(t, n)
	default:
		t.Fatal("the patched test never started, so the run was not interrupted while it blocked")
	}
}

// TestHangupLeavesTheCheckoutUnchanged runs the built program and sends it SIGHUP, as closing its
// terminal does, while a patched test blocks.
func TestHangupLeavesTheCheckoutUnchanged(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "runner")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	root := fixtureRoot(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	tmp := t.TempDir()
	before := treeDigest(t, root)
	cmd := exec.Command(bin, filepath.Join("gate", "testdata", "mutations", "interrupt.patch"))
	cmd.Dir = root
	cmd.Env = fixtureEnv("BLOCKED_FILE="+blocked, "TMPDIR="+tmp)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
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
		t.Errorf("the runner ended with %v, want exit status 1 after cleaning up:\n%s", err, out.String())
	}
	if diff := cmp.Diff(before, treeDigest(t, root), compare.Options); diff != "" {
		t.Errorf("the checkout changed (-before +after):\n%s", diff)
	}
	requireEmpty(t, tmp)
	requireExited(t, pid)
}

// TestCopyIsTheWorkingTreeAsGitSeesIt copies a tree holding a staged file made executable, an
// untracked file, an ignored file, and a staged file deleted from the working tree.
func TestCopyIsTheWorkingTreeAsGitSeesIt(t *testing.T) {
	root := fixtureRoot(t)
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored.txt\n", 0o600)
	git(t, root, "add", "--all")
	//nolint:gosec // The test needs an execute bit to see that the copy keeps it.
	if err := os.Chmod(filepath.Join(root, "gate", "gate.go"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "twin", "twin_test.go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "gate", "untracked.txt"), "untracked\n", 0o600)
	writeFile(t, filepath.Join(root, "gate", "ignored.txt"), "ignored\n", 0o600)
	files, err := workingTreeFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := copyTree(root, files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	info, err := os.Stat(filepath.Join(dir, "gate", "gate.go"))
	if err != nil {
		t.Fatalf("the staged file is not in the copy: %v", err)
	}
	if info.Mode().Perm() != 0o500 {
		t.Errorf("gate.go has mode %s in the copy, want -r-x------", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(dir, "gate", "untracked.txt")); err != nil {
		t.Errorf("the untracked file is not in the copy: %v", err)
	}
	requireAbsent(t, filepath.Join(dir, "gate", "ignored.txt"))
	requireAbsent(t, filepath.Join(dir, "twin", "twin_test.go"))
}

func TestCopyRefusesASymbolicLink(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.Symlink("gate.go", filepath.Join(root, "gate", "link")); err != nil {
		t.Fatal(err)
	}
	files, err := workingTreeFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if _, err := copyTree(root, files); err == nil || !strings.Contains(err.Error(), "gate/link is not a regular file") {
		t.Fatalf("got error %v, want the link refused", err)
	}
	requireEmpty(t, tmp)
}

// TestCopyInsideARepositoryIsPatched puts the copies under a directory that is itself a git
// repository, as a TMPDIR inside a checkout would. git apply must still patch the copy.
func TestCopyInsideARepositoryIsPatched(t *testing.T) {
	root := fixtureRoot(t)
	outer := t.TempDir()
	git(t, outer, "init", "--quiet")
	t.Setenv("TMPDIR", outer)
	res, err := demonstrateFixture(t, root, "gate", "red", fixtureEnv())
	if err != nil {
		t.Fatal(err)
	}
	if !res.held() {
		t.Errorf("the demonstration did not hold:\n%s", res.report())
	}
}

// TestCheckoutEditedDuringTheRunReachesNeitherRun edits the mechanism in the checkout while the
// unpatched tests run, and runs a patch that changes only a comment. Were the patched copy taken
// after the unpatched run, the edit would reach it alone and turn the named test red, and the
// comment would look like a removal that held.
func TestCheckoutEditedDuringTheRunReachesNeitherRun(t *testing.T) {
	root := fixtureRoot(t)
	signals := t.TempDir()
	paused, resume := filepath.Join(signals, "paused"), filepath.Join(signals, "resume")
	go func() {
		for range 1500 {
			if _, err := os.Stat(paused); err == nil {
				src, err := os.ReadFile(filepath.Join(root, "gate", "gate.go"))
				if err == nil {
					edited := strings.Replace(string(src), "return !flagged", "return true", 1)
					//nolint:gosec // The path is in the test's own temporary checkout.
					err = os.WriteFile(filepath.Join(root, "gate", "gate.go"), []byte(edited), 0o600)
				}
				if err != nil {
					t.Error(err)
				}
				writeFile(t, resume, "", 0o600)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	res, err := demonstrateFixture(t, root, "gate", "noop", fixtureEnv("PAUSED_FILE="+paused, "RESUME_FILE="+resume))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(resume); err != nil {
		t.Fatal("the checkout was never edited during the unpatched run")
	}
	want := outcome{StayedGreen: []string{"TestRefusesFlagged"}, Surviving: true}
	if diff := cmp.Diff(want, outcomeOf(res), compare.Options); diff != "" {
		t.Errorf("outcome (-want +got):\n%s", diff)
	}
}
