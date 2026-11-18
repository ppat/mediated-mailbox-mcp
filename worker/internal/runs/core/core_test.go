package core_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core"
)

// progress stands for a pass's checkpoint and counters, which each pass defines for itself.
type progress struct {
	Page     int
	Token    string
	Messages int
}

// How a run starts, from whether the pass has ended, whether it is due again because a scanner other
// than the one in force decided what the index holds, and the latest run recorded (ADR-0133,
// ADR-0120).
func TestBegin(t *testing.T) {
	at := progress{Page: 7, Token: "p8", Messages: 690}
	succeeded := core.Latest[progress]{Found: true, RunID: "r1", State: core.Succeeded, Progress: at}
	running := core.Latest[progress]{Found: true, RunID: "r1", State: core.Running, Progress: at}
	cases := []struct {
		name       string
		ended, due bool
		latest     core.Latest[progress]
		want       core.Start[progress]
	}{
		{"a pass that ended", true, false, succeeded, core.Start[progress]{Skip: true}},
		{"a pass that ended with a run left running", true, false, running, core.Start[progress]{Skip: true}},
		{"no run yet", false, false, core.Latest[progress]{}, core.Start[progress]{}},
		{"no run yet, with work another scanner decided", false, true, core.Latest[progress]{}, core.Start[progress]{}},
		{"a run that failed", false, false, core.Latest[progress]{Found: true, RunID: "r1", State: core.Failed, Progress: at}, core.Start[progress]{ResumedFrom: "r1", From: at}},
		{"a run that stopped without recording its end", false, false, running, core.Start[progress]{ResumedFrom: "r1", Abandon: true, From: at}},
		{"a pass asked to run again after it succeeded", false, false, succeeded, core.Start[progress]{}},
		{"a pass that ended and is due again", true, true, succeeded, core.Start[progress]{Reopen: true}},
		{"a pass that ended and is due again with a run left running", true, true, running, core.Start[progress]{Reopen: true, ResumedFrom: "r1", Abandon: true, From: at}},
		{"a pass not ended that is due again", false, true, running, core.Start[progress]{ResumedFrom: "r1", Abandon: true, From: at}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, core.Begin(c.ended, c.due, c.latest), compare.Options); diff != "" {
				t.Errorf("Begin (-want +got):\n%s", diff)
			}
		})
	}
}

// A job counts from its latest recorded success, from its earliest run's start while it has none,
// and from the ensure when its runs record nothing (ADR-0119).
func TestSeed(t *testing.T) {
	cases := []struct {
		name                   string
		latest, earliest, want mail.UnixMilli
	}{
		{"a success recorded", 5_000, 1_000, 5_000},
		{"runs and no success", 0, 1_000, 1_000},
		{"no run", 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := core.Seed(c.latest, c.earliest); got != c.want {
				t.Fatalf("Seed = %d, want %d", got, c.want)
			}
		})
	}
}
