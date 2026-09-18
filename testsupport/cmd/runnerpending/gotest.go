package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

// testID names one test, subtests included, in one package.
type testID struct {
	pkg, name string
}

func (id testID) String() string { return id.pkg + " " + id.name }

// testRun is the outcome of one go test run over a demonstration's packages.
type testRun struct {
	// outcome holds each test's last action, which is pass, fail or skip.
	outcome map[testID]string
	// output holds each test's printed output, shown when a run that must be green is not.
	output map[testID]string
	// brokenPackages maps a package that failed outside any test, such as by not building, to what it
	// printed.
	brokenPackages map[string]string
}

func (r testRun) failed() []testID {
	var ids []testID
	for id, action := range r.outcome {
		if action == "fail" {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b testID) int { return strings.Compare(a.String(), b.String()) })
	return ids
}

// green reports whether every package built and no test failed.
func (r testRun) green() bool {
	return len(r.failed()) == 0 && len(r.brokenPackages) == 0
}

// outcomes returns the outcomes of every test with the given name, one per package it ran in.
func (r testRun) outcomes(name string) []string {
	var got []string
	for id, action := range r.outcome {
		if id.name == name {
			got = append(got, action)
		}
	}
	return got
}

// describeRed lists the failed tests and broken packages with what they printed.
func (r testRun) describeRed() string {
	var b strings.Builder
	for _, id := range r.failed() {
		fmt.Fprintf(&b, "--- %s failed\n%s", id, r.output[id])
	}
	for _, pkg := range slices.Sorted(maps.Keys(r.brokenPackages)) {
		fmt.Fprintf(&b, "--- package %s failed outside any test\n%s", pkg, r.brokenPackages[pkg])
	}
	return b.String()
}

type testEvent struct {
	Action      string
	Package     string
	ImportPath  string
	Test        string
	Output      string
	FailedBuild string
}

// goTest runs go test -json -count=1 over packages from root with the given environment. The command
// runs in its own process group, which is killed as a whole when ctx ends, so no test binary outlives
// an interrupt.
func goTest(ctx context.Context, root string, env, packages []string) (testRun, error) {
	args := append([]string{"test", "-json", "-count=1"}, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return testRun{}, fmt.Errorf("interrupted while running go test: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return testRun{}, fmt.Errorf("running go test: %w\n%s", runErr, stderr.String())
	}

	run := testRun{outcome: map[testID]string{}, output: map[testID]string{}, brokenPackages: map[string]string{}}
	buildOutput := map[string]string{}
	events := 0
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var e testEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		events++
		id := testID{e.Package, e.Test}
		switch {
		case e.Action == "build-output":
			buildOutput[e.ImportPath] += e.Output
		case e.Test != "" && e.Action == "output":
			run.output[id] += e.Output
		case e.Test != "" && (e.Action == "pass" || e.Action == "fail" || e.Action == "skip"):
			run.outcome[id] = e.Action
		case e.Test == "" && e.Action == "output":
			buildOutput[e.Package] += e.Output
		case e.Test == "" && e.Action == "fail":
			run.brokenPackages[e.Package] = ""
			if e.FailedBuild != "" {
				run.brokenPackages[e.Package] = buildOutput[e.FailedBuild]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return testRun{}, fmt.Errorf("reading go test output: %w", err)
	}
	if runErr != nil && events == 0 {
		return testRun{}, fmt.Errorf("go test failed before running anything: %w\n%s", runErr, stderr.String())
	}
	// A package fails outside any test only when no test in it failed. A package whose failure a test
	// explains is not broken.
	for pkg := range run.brokenPackages {
		for id, action := range run.outcome {
			if id.pkg == pkg && action == "fail" {
				delete(run.brokenPackages, pkg)
				break
			}
		}
	}
	for pkg, out := range run.brokenPackages {
		if out == "" {
			run.brokenPackages[pkg] = buildOutput[pkg] + stderr.String()
		}
	}
	return run, nil
}
