//go:build integration

package policyload_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
)

// reloadDurationName is the series timing each reload (ADR-0125).
const reloadDurationName = "mediated_mailbox_policyload_reload_duration_seconds"

// reloadsTimed returns how many reloads the reload duration series on reg has observed.
func reloadsTimed(t *testing.T, reg prometheus.Gatherer) uint64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == reloadDurationName {
			if n := len(f.GetMetric()); n != 1 {
				t.Fatalf("%s has %d series, want one", reloadDurationName, n)
			}
			return f.GetMetric()[0].GetHistogram().GetSampleCount()
		}
	}
	t.Fatalf("%s is not registered", reloadDurationName)
	return 0
}

// Every reload its caller did not cancel is timed, one that succeeds, one whose update does not
// validate and one past its deadline, and a reload its caller cancelled is not, since its time says
// nothing of the policy's cost (ADR-0125).
func TestEachReloadItsCallerDidNotCancelIsTimed(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn, 2)
	seed(t, conn, accounts)
	l, reg := newLoader(t, &faulty{pool: backfill(t)}, accounts)
	if got := reloadsTimed(t, reg); got != 0 {
		t.Fatalf("reloads timed before the first = %d, want 0", got)
	}
	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := reloadsTimed(t, reg); got != 1 {
		t.Errorf("reloads timed after one succeeded = %d, want 1", got)
	}
	addRule(t, conn, "", "base.typo", "example com")
	if err := l.Reload(t.Context()); !errors.Is(err, policyload.ErrInvalidUpdate) {
		t.Fatalf("a reload of an invalid update returned %v, want ErrInvalidUpdate", err)
	}
	if got := reloadsTimed(t, reg); got != 2 {
		t.Errorf("reloads timed after an invalid update = %d, want 2", got)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := l.Reload(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled reload returned %v, want context.Canceled", err)
	}
	if got := reloadsTimed(t, reg); got != 2 {
		t.Errorf("reloads timed after a cancelled reload = %d, want 2", got)
	}
	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	if err := l.Reload(expired); !errors.Is(err, policyload.ErrUntrustedRead) {
		t.Fatalf("a reload past its deadline returned %v, want ErrUntrustedRead", err)
	}
	if got := reloadsTimed(t, reg); got != 3 {
		t.Errorf("reloads timed after a reload past its deadline = %d, want 3", got)
	}
}
