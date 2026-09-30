package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

// A patch whose control the browser's tests carry names runner bun, and its demonstration runs bun
// test instead of go test (ADR-0064). Its packages are bun test files relative to the repository root,
// all under one browser package, the nearest directory above them holding a package.json, and bun
// test runs from that directory. Its tests are the test names bun reports, a test inside a describe
// block written as the block's name, " > " and the test's name, and since a test name holds spaces
// the names are separated by " | ".
//
// A copy of the working tree holds no node_modules, which git ignores. The browser package's
// node_modules in the checkout is linked into each copy, so bun resolves the packages the checkout
// installed. bun reads them and writes nothing there.

// bunPackage returns the browser package directory, relative to the root, that holds every test file
// in files, and the files relative to it. It refuses files in more than one package.
func bunPackage(root string, files []string) (dir string, rel []string, err error) {
	for _, f := range files {
		d, err := packageDir(root, filepath.Dir(filepath.Clean(f)))
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", f, err)
		}
		if dir != "" && d != dir {
			return "", nil, fmt.Errorf("the test files are in two browser packages, %s and %s", dir, d)
		}
		dir = d
		r, err := filepath.Rel(d, filepath.Clean(f))
		if err != nil {
			return "", nil, err
		}
		rel = append(rel, r)
	}
	return dir, rel, nil
}

// packageDir walks up from dir to the nearest directory holding a package.json, below root.
func packageDir(root, dir string) (string, error) {
	for d := dir; d != "." && d != "/" && d != ""; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(root, d, "package.json")); err == nil {
			return d, nil
		}
	}
	return "", errors.New("no directory above it holds a package.json")
}

// linkModules links the checkout's node_modules of the browser package into a copy, when the checkout
// has one.
func linkModules(checkout, copyRoot, dir string) error {
	modules := filepath.Join(checkout, dir, "node_modules")
	if _, err := os.Stat(modules); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Symlink(modules, filepath.Join(copyRoot, dir, "node_modules"))
}

type junitReport struct {
	Suites []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	File   string       `xml:"file,attr"`
	Suites []junitSuite `xml:"testsuite"`
	Cases  []junitCase  `xml:"testcase"`
}

type junitCase struct {
	Name      string    `xml:"name,attr"`
	ClassName string    `xml:"classname,attr"`
	File      string    `xml:"file,attr"`
	Failure   *struct{} `xml:"failure"`
	Skipped   *struct{} `xml:"skipped"`
}

// parseJUnit reads bun test's JUnit report into a run, each test keyed by its file and its name.
func parseJUnit(src []byte) (testRun, error) {
	var report junitReport
	if err := xml.Unmarshal(src, &report); err != nil {
		return testRun{}, fmt.Errorf("reading bun test's report: %w", err)
	}
	run := testRun{outcome: map[testID]string{}, output: map[testID]string{}, brokenPackages: map[string]string{}}
	var walk func(suites []junitSuite)
	walk = func(suites []junitSuite) {
		for _, s := range suites {
			for _, c := range s.Cases {
				name := c.Name
				if c.ClassName != "" {
					name = c.ClassName + " > " + c.Name
				}
				action := "pass"
				switch {
				case c.Failure != nil:
					action = "fail"
				case c.Skipped != nil:
					action = "skip"
				}
				run.outcome[testID{c.File, name}] = action
			}
			walk(s.Suites)
		}
	}
	walk(report.Suites)
	return run, nil
}

// bunTest runs the patch's test files with bun test from the browser package in root, a copy of the
// working tree, and reads the outcome from its JUnit report. bun runs in its own process group,
// which is killed when ctx ends. A run that fails with no failing test is a broken package, such as a
// test file that does not load.
func bunTest(ctx context.Context, checkout, root string, env []string, files []string) (testRun, error) {
	dir, rel, err := bunPackage(root, files)
	if err != nil {
		return testRun{}, err
	}
	if err := linkModules(checkout, root, dir); err != nil {
		return testRun{}, err
	}
	report := filepath.Join(root, ".bun-junit.xml")
	args := append([]string{"test", "--reporter=junit", "--reporter-outfile=" + report}, rel...)
	cmd := exec.CommandContext(ctx, "bun", args...)
	cmd.Dir = filepath.Join(root, dir)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	out, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return testRun{}, fmt.Errorf("interrupted while running bun test: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return testRun{}, fmt.Errorf("running bun test: %w\n%s", runErr, out)
	}
	src, err := os.ReadFile(report) //nolint:gosec // The report is written by bun into the copy.
	if errors.Is(err, os.ErrNotExist) && runErr != nil {
		// bun writes no report when a test file fails to load, so the package is broken.
		return testRun{outcome: map[testID]string{}, output: map[testID]string{}, brokenPackages: map[string]string{dir: string(out)}}, nil
	}
	if err != nil {
		return testRun{}, fmt.Errorf("bun test wrote no report: %w\n%s", err, out)
	}
	run, err := parseJUnit(src)
	if err != nil {
		return testRun{}, err
	}
	if runErr != nil && len(run.failed()) == 0 {
		run.brokenPackages[dir] = string(out)
	}
	for _, id := range run.failed() {
		run.output[id] = string(out)
	}
	return run, nil
}

// bunTestNames splits a bun patch's tests value on " | ".
func bunTestNames(value string) []string {
	var names []string
	for name := range strings.SplitSeq(value, " | ") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return slices.Clip(names)
}
