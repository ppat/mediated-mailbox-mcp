//go:build integration

package postgres_test

import (
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// withoutPgrun is this process's environment without the variables pgrun sets, so a command run with
// it starts as if pgrun had never run.
func withoutPgrun() []string {
	return slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, postgres.EnvAdminURL+"=") || strings.HasPrefix(kv, postgres.EnvTemplate+"=")
	})
}

// TestPackageFailsWithoutPgrun starts an integration test package with the tag but without the
// database pgrun provides. Main must fail the package with its own message, which tells this failure
// apart from a build error or a refused connection.
func TestPackageFailsWithoutPgrun(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", "-tags=integration", "./testdata/isolation/first")
	cmd.Env = withoutPgrun()
	out, err := cmd.CombinedOutput()
	t.Logf("go test ./testdata/isolation/first, without pgrun\n%s", out)
	if err == nil {
		t.Fatal("the integration test package passed without the database pgrun provides")
	}
	if !strings.Contains(string(out), "integration tests need the database pgrun starts") {
		t.Fatalf("the package failed, but not because pgrun's variables were missing: %v", err)
	}
}

// TestPgrunFailsWithoutTag runs pgrun over go test without the integration tag. pgrun is started from
// the repository root, where it must run, with a container of its own on the next port after this
// run's, reached at the same host. With a remote docker daemon that port needs a route as well as
// the outer run's.
//
// Without the tag the isolation packages build no file, and testsupport/postgres keeps no test, so go
// test exits zero having run nothing. The explicit package keeps the run from matching no package at
// all, which go test itself refuses. pgrun prints the message below only after the command exited
// zero, so the failure is its check that no test package created a database.
func TestPgrunFailsWithoutTag(t *testing.T) {
	admin, err := neturl.Parse(os.Getenv(postgres.EnvAdminURL))
	if err != nil {
		t.Fatalf("%s is not a URL: %v", postgres.EnvAdminURL, err)
	}
	host, port, err := net.SplitHostPort(admin.Host)
	if err != nil {
		t.Fatalf("%s names no host and port: %v", postgres.EnvAdminURL, err)
	}
	outer, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "tool", "pgrun", "-host", host, "-port", strconv.Itoa(outer+1), "--", //nolint:gosec // the host is the outer pgrun's own, passed to no shell
		"go", "test", "-count=1", "./testsupport/postgres", "./testsupport/postgres/testdata/isolation/...")
	cmd.Dir = "../.."
	cmd.Env = withoutPgrun()
	out, err := cmd.CombinedOutput()
	t.Logf("go tool pgrun -- go test, without the integration tag\n%s", out)
	if err == nil {
		t.Fatal("pgrun passed a run in which no integration test ran")
	}
	if !strings.Contains(string(out), "no test package created a database") {
		t.Fatalf("pgrun failed, but not because no test package created a database: %v. With a remote docker daemon, port %d on %s needs a route as well as port %d", err, outer+1, host, outer)
	}
}
