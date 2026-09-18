// Command runnerpending runs mutation demonstrations, the proof ADR-0046 requires of an automatable
// control's tests. A demonstration removes the control's mechanism with a checked-in patch, requires
// the tests it names to go red, and records which tests went red for the ledger in docs/MUTATIONS.md.
//
// Each patch is one mutant and is applied alone. A control whose mechanism is broken more than one
// way, such as a break that makes the code do less and a break that makes it do the wrong thing,
// has one patch per break, because two breaks applied together can hide each other. A patch is a
// .patch file in testdata/mutations in the package whose mechanism it removes, which pre-commit's
// fixers leave byte for byte. It describes itself in the free text before its first diff header,
// which git apply ignores. The preamble holds exactly these keys, one per line.
//
//	control: the control, as the ledger names it
//	removes: one line saying how the patch removes or disables the mechanism
//	packages: package paths relative to the repository root, such as ./core/redact or ./core/...
//	tests: the tests that must go red, by the names go test reports, subtests included
//	scheduled-count: yes when the demonstration goes red only at the scheduled case count, else no
//	diff --git a/core/redact/redact.go b/core/redact/redact.go
//	...
//
// A package entry that is not a relative path is refused, because it would reach go test as a flag.
// A patch whose file names mark it as touching test code, a _test.go file or anything under a
// testdata directory, is refused, because a red from a changed test says nothing about the mechanism.
// Whether a patch removes the control's mechanism rather than test tooling the tests rely on, such
// as a shared comparison option, is for the review of the patch, which ADR-0046 makes the checked-in
// artifact.
//
// For each patch the runner does four things.
//
//  1. It runs the packages' tests unpatched and requires them green, and requires every named test
//     to have passed there, so a red in the next step is the patch's doing and a misspelled or
//     skipped test is caught rather than silently unmet.
//  2. It records the state of the whole working tree, saves every file the patch creates, changes,
//     renames or deletes, applies the patch with git apply, runs the same tests, and records every
//     test that failed. A package that fails without any test failing, such as one the patch stops
//     from building, is reported and fails the demonstration, because a patch that breaks the build
//     proves nothing about the tests.
//  3. It writes the saved files back with their modes on every path out of the run, interrupts
//     included, and checks them byte for byte. It then compares the whole working tree with its
//     state before the patch, and a difference, such as a file the patched code wrote, fails the
//     demonstration and names the paths. The comparison also runs when the run was interrupted.
//  4. It runs the tests again and requires them green.
//
// The demonstration holds when every named test went red in every package it ran in. A patch that
// leaves every test green is reported as a surviving mutant, which ADR-0046 counts as a defect on the
// spot. After the last patch the runner prints one ledger row per control, as docs/MUTATIONS.md
// defines a row, naming each removal and the tests it turned red, and a surviving mutant marks the
// row open. A control gets a row only when every one of its patches given in the run held or
// survived. When one failed, could not be judged, or never ran because the run stopped, the runner
// prints no row and says the control's demonstration is incomplete. The evidence pointer is left to
// the author. The run stops at a failed restore or a working tree that differs, since no
// later patch can run on a tree that is not as it was.
//
// Every run sets RAPID_NOFAILFILE=true, so a red property never writes a fail file under
// testdata/rapid, which ADR-0069 forbids keeping. RAPID_CHECKS passes through from the environment,
// and -short is never passed, since it divides rapid's case count by five. The runner refuses to run
// while GOFLAGS is set, in the environment or through go env -w, because go test reads its flags
// from there and any of them, such as -run or -short, changes which tests run or how. A patch with scheduled-count yes
// runs with RAPID_CHECKS set from RAPID_SCHEDULED_CHECKS, the count the deep-tests workflow reads
// from the repository variable of that name, because a rare failure can be reached at the gating
// count only by luck (ADR-0069). The runner refuses such a patch when RAPID_SCHEDULED_CHECKS is not
// a positive number. RAPID_SEED passes through when it is set and non-zero. Otherwise the runner
// picks a seed. All three runs of a patch use the same seed, and the report records the seed and
// the case count, so a property demonstration can be repeated.
//
// SIGINT, SIGTERM and SIGHUP stop the run and restore the patched files. The runner runs ordinary go
// test packages, property tests included. It is run by hand when a control lands or changes, never
// as a standing gate. Run it from the repository root with the patches as arguments.
//
//	go tool runnerpending core/redact/testdata/mutations/*.patch
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: go tool runnerpending patch...")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err == nil {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr != nil {
			err = errors.New("run from the repository root, where go.mod is")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "runnerpending:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if !runAll(ctx, os.Stdout, root, os.Environ(), flag.Args()) {
		stop()
		os.Exit(1)
	}
}

// runAll runs every patch in turn, then prints the ledger rows, and reports whether every
// demonstration held. It stops at the first patch that leaves the working tree in doubt or ends the
// run, and carries on past any other failure.
func runAll(ctx context.Context, w io.Writer, root string, environ, patches []string) bool {
	// Every patch's control is read first, so a patch that errs or never runs still counts against
	// its control's row.
	entries := make([]ledgerEntry, len(patches))
	for i, patch := range patches {
		if src, err := os.ReadFile(patch); err == nil {
			if p, err := parsePatch(patch, src); err == nil {
				entries[i].control = p.control
			}
		}
	}
	ok := true
	for i, patch := range patches {
		res, err := demonstrate(ctx, root, environ, patch)
		if err != nil {
			ok = false
			if _, writeErr := fmt.Fprintf(w, "FAIL %s\n%v\n\n", patch, err); writeErr != nil {
				return false
			}
			var stop *stopError
			if errors.As(err, &stop) || ctx.Err() != nil {
				break
			}
			continue
		}
		if _, err := io.WriteString(w, res.report()); err != nil {
			return false
		}
		entries[i].res = &res
		ok = ok && res.held()
		if len(res.strays) > 0 {
			break
		}
	}
	rows, incomplete := ledgerRows(entries)
	var b strings.Builder
	if len(rows) > 0 {
		fmt.Fprintf(&b, "Ledger rows for docs/MUTATIONS.md\n%s\n", strings.Join(rows, "\n"))
	}
	for _, c := range incomplete {
		fmt.Fprintf(&b, "No ledger row for %q, because its demonstration is incomplete. A patch of it did not hold or survive, or never ran\n", c)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return false
	}
	return ok
}

// stopError marks a failure to restore the patched files, after which no further patch may run,
// because the working tree is not as it was.
type stopError struct{ err error }

func (e *stopError) Error() string { return e.err.Error() }
func (e *stopError) Unwrap() error { return e.err }

// result is the outcome of one demonstration that ran to the end.
type result struct {
	patch    string
	preamble preamble
	date     time.Time
	// seed and checks are the RAPID_SEED and RAPID_CHECKS every run of the patch had, checks empty
	// when unset.
	seed, checks string
	// red lists every test that failed under the patch.
	red []testID
	// broken lists the packages that failed under the patch without a failing test, with their output.
	broken map[string]string
	// stayedGreen lists the required tests that did not fail in every package they ran in under the
	// patch, including those that did not run at all.
	stayedGreen []string
	// strays lists the paths whose state after the restore differs from before the patch.
	strays []string
	// notRestored describes the tests that were red after the patch was removed.
	notRestored string
}

func (r result) surviving() bool { return len(r.red) == 0 && len(r.broken) == 0 }

func (r result) held() bool {
	return !r.surviving() && len(r.broken) == 0 && len(r.stayedGreen) == 0 && len(r.strays) == 0 && r.notRestored == ""
}

// demonstrate runs one patch through the four steps in the package comment. An error means the
// demonstration could not be judged. A judged demonstration that failed is a result that did not
// hold.
func demonstrate(ctx context.Context, root string, environ []string, patchPath string) (result, error) {
	src, err := os.ReadFile(patchPath)
	if err != nil {
		return result{}, err
	}
	p, err := parsePatch(patchPath, src)
	if err != nil {
		return result{}, err
	}
	env, seed, checks, err := testEnv(environ, p.scheduled)
	if err != nil {
		return result{}, err
	}
	absPatch, err := filepath.Abs(patchPath)
	if err != nil {
		return result{}, err
	}
	if _, err := gitApply(root, "--check", absPatch); err != nil {
		return result{}, fmt.Errorf("the patch does not apply: %w", err)
	}
	paths, err := touchedPaths(root, absPatch, src)
	if err != nil {
		return result{}, err
	}
	if tests := testCodePaths(paths); len(tests) > 0 {
		return result{}, fmt.Errorf("the patch touches test code, so a red under it says nothing about the mechanism: %s", strings.Join(tests, ", "))
	}

	baseline, err := goTest(ctx, root, env, p.packages)
	if err != nil {
		return result{}, err
	}
	if !baseline.green() {
		return result{}, fmt.Errorf("the tests are red before the patch is applied, so a red under it proves nothing\n%s", baseline.describeRed())
	}
	var absent []string
	for _, name := range p.tests {
		outcomes := baseline.outcomes(name)
		if len(outcomes) == 0 || slices.ContainsFunc(outcomes, func(o string) bool { return o != "pass" }) {
			absent = append(absent, name)
		}
	}
	if len(absent) > 0 {
		return result{}, fmt.Errorf("these tests did not run and pass before the patch was applied: %s", strings.Join(absent, ", "))
	}

	res := result{patch: patchPath, preamble: p, date: time.Now(), seed: seed, checks: checks}
	before, err := treeState(root)
	if err != nil {
		return result{}, err
	}
	mutant, err := runPatched(ctx, root, env, absPatch, paths, p.packages)
	if err != nil {
		// The restore has run. Files the patched code wrote before the run ended are named too.
		if state, stateErr := treeState(root); stateErr != nil {
			err = fmt.Errorf("%w\nthe working tree was not checked against its state before the patch: %w", err, stateErr)
		} else if strays := changedPaths(before, state); len(strays) > 0 {
			err = fmt.Errorf("%w\nthe working tree differs from before the patch at %s", err, strings.Join(strays, ", "))
		}
		return result{}, err
	}
	res.red = mutant.failed()
	res.broken = mutant.brokenPackages
	for _, name := range p.tests {
		outcomes := mutant.outcomes(name)
		if len(outcomes) == 0 || slices.ContainsFunc(outcomes, func(o string) bool { return o != "fail" }) {
			res.stayedGreen = append(res.stayedGreen, name)
		}
	}
	afterState, err := treeState(root)
	if err != nil {
		return result{}, &stopError{err}
	}
	if res.strays = changedPaths(before, afterState); len(res.strays) > 0 {
		return res, nil
	}

	after, err := goTest(ctx, root, env, p.packages)
	if err != nil {
		return result{}, err
	}
	if !after.green() {
		res.notRestored = after.describeRed()
	}
	return res, nil
}

// runPatched applies the patch, runs the tests, and restores the files the patch touches, on every
// path out of it.
func runPatched(ctx context.Context, root string, env []string, absPatch string, paths, packages []string) (run testRun, err error) {
	snap, err := takeSnapshot(root, paths)
	if err != nil {
		return testRun{}, err
	}
	defer func() {
		if restoreErr := snap.restore(); restoreErr != nil {
			err = &stopError{errors.Join(err, restoreErr)}
		}
	}()
	if _, err := gitApply(root, absPatch); err != nil {
		return testRun{}, err
	}
	return goTest(ctx, root, env, packages)
}

// testEnv builds the environment every go test run of a patch gets from the runner's own
// environment, and returns the RAPID_SEED and RAPID_CHECKS in it, checks empty when unset.
func testEnv(environ []string, scheduled bool) (env []string, seed, checks string, err error) {
	goflags, err := effectiveGOFLAGS(environ)
	if err != nil {
		return nil, "", "", err
	}
	if goflags != "" {
		return nil, "", "", fmt.Errorf("GOFLAGS is %q, and any go test flag there changes which tests run or how, so unset it (go env -u GOFLAGS as well, if go env -w set it)", goflags)
	}
	seed, checks = lookup(environ, "RAPID_SEED"), lookup(environ, "RAPID_CHECKS")
	if scheduled {
		checks = lookup(environ, "RAPID_SCHEDULED_CHECKS")
		if n, err := strconv.Atoi(checks); err != nil || n <= 0 {
			return nil, "", "", fmt.Errorf("the patch relies on the scheduled case count, so RAPID_SCHEDULED_CHECKS must be a positive number, and it is %q", checks)
		}
	}
	if n, err := strconv.ParseUint(seed, 10, 64); err != nil || n == 0 {
		// Every run of the patch gets this one seed, so the report can say how to repeat the runs.
		var b [8]byte
		_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
		seed = strconv.FormatUint(binary.LittleEndian.Uint64(b[:])>>2+1, 10)
	}
	env = slices.DeleteFunc(slices.Clone(environ), func(kv string) bool {
		return strings.HasPrefix(kv, "RAPID_NOFAILFILE=") || strings.HasPrefix(kv, "RAPID_SEED=") || strings.HasPrefix(kv, "RAPID_CHECKS=")
	})
	env = append(env, "RAPID_NOFAILFILE=true", "RAPID_SEED="+seed)
	if checks != "" {
		env = append(env, "RAPID_CHECKS="+checks)
	}
	return env, seed, checks, nil
}

// lookup returns the last value environ gives the variable, as a process sees it.
func lookup(environ []string, name string) string {
	value := ""
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, name+"="); ok {
			value = v
		}
	}
	return value
}

// effectiveGOFLAGS returns the GOFLAGS go test would read, from the environment or from the file go
// env -w writes.
func effectiveGOFLAGS(environ []string) (string, error) {
	cmd := exec.Command("go", "env", "GOFLAGS")
	cmd.Env = environ
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOFLAGS: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
