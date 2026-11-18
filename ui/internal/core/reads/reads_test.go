package reads_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/reads"
)

// A workload with no run is not started, one with a run in running is running whatever its other
// passes' runs, and one whose runs have all finished is idle, and the System screen counts each
// workload with a run in running once (docs/UI.md sections 8.3, 8.9 and 11, VERIFICATIONS, the jobs
// cards row).
func TestAWorkloadsStateIsDerivedFromItsLatestRuns(t *testing.T) {
	runs := []reads.Run{
		{Workload: "backfill", State: "succeeded"},
		{Workload: "backfill", State: "running"},
		{Workload: "sync", State: "failed"},
		{Workload: "sync", State: "succeeded"},
		{Workload: "apply", State: "running"},
		{Workload: "apply", State: "running"},
	}
	for workload, want := range map[string]string{"backfill": reads.Running, "sync": reads.Idle, "apply": reads.Running, "heuristics": reads.NotStarted} {
		if got := reads.WorkloadState(runs, workload); got != want {
			t.Errorf("%s: %s, want %s", workload, got, want)
		}
	}
	if got := reads.RunningWorkloads(runs); got != 2 {
		t.Errorf("%d workloads running, want 2", got)
	}
	if got := reads.RunningWorkloads(nil); got != 0 {
		t.Errorf("%d workloads running with no run, want 0", got)
	}
}

// An apply can be rolled back once its plan is APPLIED with an identifier, and the next heuristics run
// is due a cadence after the last started (docs/UI.md section 8.3).
func TestAnApplysRollbackAndTheNextRunAreDecidedFromTheLastRun(t *testing.T) {
	applied, draft, id := "APPLIED", "APPROVED", "a-plan"
	for name, c := range map[string]struct {
		status, id *string
		want       bool
	}{
		"applied":       {&applied, &id, true},
		"another state": {&draft, &id, false},
		"no plan":       {&applied, nil, false},
		"no status":     {nil, &id, false},
	} {
		if got := reads.RollbackAvailable(c.status, c.id); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	if got := reads.NextRun(1_000, 86_400); got != 87_400 {
		t.Errorf("the next run is due at %d", got)
	}
}

// A running pass has a time left once it has a window of history, the pages remaining over the pages
// completed in the window, and none when the window completed no page (docs/UI.md section 8.1,
// VERIFICATIONS, the estimated time left row).
func TestARunningPassesTimeLeftIsItsPagesOverTheWindowsPace(t *testing.T) {
	const window = int64(600)
	for name, c := range map[string]struct {
		state        string
		started, now int64
		want         bool
	}{
		"running a window":       {"running", 0, 600, true},
		"running longer":         {"running", 0, 900, true},
		"running less than that": {"running", 1, 600, false},
		"finished":               {"succeeded", 0, 900, false},
	} {
		if got := reads.Estimable(c.state, c.started, c.now, window); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	at := func(p *int64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	for name, c := range map[string]struct {
		page, of, events, done int64
		want                   any
	}{
		"half done":       {50, 100, 3, 10, int64(3000)},
		"past its end":    {120, 100, 3, 10, int64(0)},
		"no event":        {50, 100, 0, 10, nil},
		"no page done":    {50, 100, 3, 0, nil},
		"pages gone back": {50, 100, 3, -2, nil},
	} {
		if got := at(reads.TimeLeft(c.page, c.of, c.events, c.done, window)); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

// The stream sends an object's event when what it reads differs from what it last sent, and the
// first time it reads one (docs/UI.md section 17.5).
func TestTheStreamSendsWhatChanged(t *testing.T) {
	for name, c := range map[string]struct {
		sent, read []byte
		want       bool
	}{
		"unsent":    {nil, []byte(`{"a":1}`), true},
		"unchanged": {[]byte(`{"a":1}`), []byte(`{"a":1}`), false},
		"changed":   {[]byte(`{"a":1}`), []byte(`{"a":2}`), true},
	} {
		if got := reads.Changed(c.sent, c.read); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}
