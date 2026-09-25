//go:build unix

package gmail_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The file size limit this test relies on is a Unix resource limit, and Go ignores the SIGXFSZ
// the limit raises, so the write returns an error rather than killing the child.

// The environment variables that turn this test binary into the child of
// TestAFailedWriteLeavesTheLocationAsItWas.
const (
	childPath  = "MEDIATED_MAILBOX_WRITE_BACK_CHILD_PATH"
	childToken = "MEDIATED_MAILBOX_WRITE_BACK_CHILD_TOKEN"
	childLimit = "MEDIATED_MAILBOX_WRITE_BACK_CHILD_FILE_SIZE_LIMIT"
)

func TestMain(m *testing.M) {
	if path := os.Getenv(childPath); path != "" {
		os.Exit(writeBackUnderAFileSizeLimit(path))
	}
	os.Exit(m.Run())
}

// writeBackUnderAFileSizeLimit caps the size of any file this process writes, then writes back a
// token longer than the cap. The write fails part of the way through, as it would on a full disk.
// Exit status 0 means the write failed as intended.
func writeBackUnderAFileSizeLimit(path string) int {
	limit, err := strconv.ParseUint(os.Getenv(childLimit), 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &rl); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	rl.Cur = limit
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &rl); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := gmail.WriteBack(path, os.Getenv(childToken)); err == nil {
		fmt.Fprintln(os.Stderr, "the write succeeded under the file size limit")
		return 3
	}
	return 0
}

// A write that fails part of the way through leaves the location holding its previous content, so
// the next start never reads part of a token. The child process writes under a file size limit,
// which fails the write after its first bytes land, the way a full disk does.
func TestAFailedWriteLeavesTheLocationAsItWas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-token")
	const previous = "previous-refresh-token"
	if err := os.WriteFile(path, []byte(previous), 0o600); err != nil {
		t.Fatal(err)
	}
	rotated := strings.Repeat("r", 4*len(previous))

	cmd := exec.Command(os.Args[0], "-test.run=^$") //nolint:gosec // re-runs this test binary as the child
	cmd.Env = append(os.Environ(),
		childPath+"="+path,
		childToken+"="+rotated,
		childLimit+"="+strconv.Itoa(2*len(previous)),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("the child did not fail its write as intended, exit status %d: %s", exit.ExitCode(), stderr.String())
		}
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != previous {
		t.Errorf("after a failed write the location holds %q, want %q", got, previous)
	}
	if diff := cmp.Diff([]string{"refresh-token"}, dirEntries(t, dir), compare.Options); diff != "" {
		t.Errorf("directory entries (-want +got):\n%s", diff)
	}
}
