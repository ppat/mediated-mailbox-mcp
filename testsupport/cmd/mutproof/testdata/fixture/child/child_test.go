// Package child_test starts a go command the way a demonstration's tests do, so the runner's tests
// can require that the command gets -trimpath.
package child_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestAStartedGoCommandGetsTrimpath passes only when a go command this test starts reads -trimpath
// from its GOFLAGS.
func TestAStartedGoCommandGetsTrimpath(t *testing.T) {
	out, err := exec.Command("go", "env", "GOFLAGS").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Fields(string(out)), "-trimpath") {
		t.Fatalf("go env GOFLAGS printed %q, want -trimpath", out)
	}
}
