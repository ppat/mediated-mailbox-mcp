package fake_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/contract"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
)

// reserveChild names the variable under which the test runs the slowed suite itself, as the child
// process the test starts.
const reserveChild = "MEDIATED_MAILBOX_RESERVE_CONTRACT_CHILD"

// The markers the child writes, a case's cleanup running and the suite stopping inside a page loop
// and before the next case.
const (
	cleanupRan  = "the case's cleanup ran"
	stoppedLoop = "before the test's deadline, so the case's cleanups run"
	stoppedRun  = "cases short of the end"
)

// slowEnumeration is a fake whose every enumeration page takes a second and a half, so one case's
// enumeration outlasts the gap between the test's start and its reserve.
type slowEnumeration struct{ *fake.Fake }

func (s slowEnumeration) EnumerateAll(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	time.Sleep(1500 * time.Millisecond)
	return s.Fake.EnumerateAll(ctx, page)
}

// A suite whose reserve is near stops inside the page loop of the case it is running and starts no
// other case, so the case's cleanups run and the test binary is never stopped at its deadline. The
// child runs with a six-second timeout and a four-second reserve, and its first case pages slowly.
func TestTheSuiteStopsAReserveBeforeItsDeadline(t *testing.T) {
	if os.Getenv(reserveChild) == "1" {
		contract.Run(t, contract.Config{
			Harness: func(t *testing.T, account string) contract.Implementation {
				f := newFake(t, account, nil)
				t.Cleanup(func() { t.Log(cleanupRan) })
				return contract.Implementation{
					Port: slowEnumeration{f},
					Deliver: func(m contract.Message) (string, error) {
						return m.Metadata.ID, f.Deliver(fake.Message{Metadata: m.Metadata, Body: m.Body})
					},
					Remove: f.Remove,
				}
			},
			Run:     "fake",
			Reserve: 4 * time.Second,
		})
		return
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestTheSuiteStopsAReserveBeforeItsDeadline$", "-test.v", "-test.count=1", "-test.timeout=6s") //nolint:gosec // the test binary running itself
	cmd.Env = append(os.Environ(), reserveChild+"=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the slowed suite passed, and it should have stopped before its deadline:\n%s", out)
	}
	for _, want := range []string{stoppedLoop, stoppedRun, cleanupRan} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("the slowed suite's output lacks %q:\n%s", want, out)
		}
	}
	if bytes.Contains(out, []byte("panic: test timed out")) {
		t.Errorf("the slowed suite ran into its deadline:\n%s", out)
	}
}
