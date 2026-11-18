// Package alertingrules holds what a package's test of its alerting rules shares, the run of
// promtool's unit tests over the rules file and the check that every series the rules read is one the
// package emits (ADR-0076, ADR-0077). A missing promtool fails the test rather than skipping it,
// because a skipped test passes in the job that was meant to run it.
package alertingrules

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"testing"
)

// Test runs `promtool test rules` over testFile, a promtool unit test file of the chart's alerting
// rules, and fails t with promtool's output when a case fails.
func Test(t *testing.T, testFile string) {
	t.Helper()
	path, err := exec.LookPath("promtool")
	if err != nil {
		t.Fatalf("promtool is not on PATH. Run the tests through mise: %v", err)
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(t.Context(), path, "test", "rules", testFile)
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		t.Errorf("promtool test rules failed: %v\n%s", err, out.String())
	case err != nil:
		t.Fatalf("running promtool: %v", err)
	}
}

// RequireReadOnlyEmitted fails t for each series name matching pattern that the rules file reads and
// emitted does not hold, so no rule watches a name nothing emits. It fails too when the rules read
// fewer than atLeast distinct names, since a pattern that matches nothing proves nothing.
func RequireReadOnlyEmitted(t *testing.T, rulesFile, pattern string, atLeast int, emitted map[string]bool) {
	t.Helper()
	src, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	read := slices.Compact(slices.Sorted(slices.Values(regexp.MustCompile(pattern).FindAllString(string(src), -1))))
	if len(read) < atLeast {
		t.Fatalf("the rules read %d series matching %s, want at least %d: %v", len(read), pattern, atLeast, read)
	}
	for _, name := range read {
		if !emitted[name] {
			t.Errorf("the rules read %s, which nothing emits", name)
		}
	}
}
