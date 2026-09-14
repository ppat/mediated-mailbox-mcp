//go:build integration

package postgres_test

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/ppat/mediated-mailbox-mcp"

// TestPackagesGetTheirOwnDatabase runs the two identical integration test packages under
// testdata/isolation, in parallel, against the container pgrun started for this run. Each creates a
// table no migration creates, in a database that must start without it, so whichever ran second would
// find the table if the two shared a database. Go's package patterns skip testdata, so the packages run
// only here.
func TestPackagesGetTheirOwnDatabase(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", "-p=2", "-tags=integration", "./testdata/isolation/...")
	out, err := cmd.CombinedOutput()
	t.Logf("go test ./testdata/isolation/...\n%s", out)
	if err != nil {
		t.Fatalf("the isolation packages failed: %v", err)
	}
	// A pattern matching no package also exits zero, so both packages must report.
	for _, name := range []string{"first", "second"} {
		if !strings.Contains(string(out), "ok  \t"+module+"/testsupport/postgres/testdata/isolation/"+name) {
			t.Errorf("the isolation package %s did not run", name)
		}
	}
}
