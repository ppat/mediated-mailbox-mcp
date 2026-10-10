package due_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due"
)

// The run-start step is due until this process has made it, and the passes while either is not
// ended or the second is marked to start over (ADR-0119, ADR-0121).
func TestBackfillsDueDecision(t *testing.T) {
	ended := due.State{FirstEnded: true, SecondEnded: true}
	cases := []struct {
		name    string
		state   due.State
		started bool
		want    due.Work
	}{
		{"a new account", due.State{}, false, due.Work{Start: true, Passes: true}},
		{"both passes ended, not yet started in this process", ended, false, due.Work{Start: true, Passes: true}},
		{"both passes ended and started", ended, true, due.Work{}},
		{"the first pass not ended", due.State{SecondEnded: true}, true, due.Work{Passes: true}},
		{"the second pass not ended", due.State{FirstEnded: true}, true, due.Work{Passes: true}},
		{"the second pass marked to start over", due.State{FirstEnded: true, SecondEnded: true, Restart: true}, true, due.Work{Passes: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := due.Decide(c.state, c.started)
			if got != c.want {
				t.Fatalf("Decide = %+v, want %+v", got, c.want)
			}
			if got.Any() != (c.want != due.Work{}) {
				t.Fatalf("Any = %v for %+v", got.Any(), got)
			}
		})
	}
}

// A backfill job with work outstanding counts from its latest recorded success, and from its earliest
// run's start while it has none, so a first pass in a crash loop ages from its first attempt. One with
// no work outstanding, and one whose runs record nothing, count from the ensure (ADR-0119).
func TestBackfillsSeed(t *testing.T) {
	ended := due.State{FirstEnded: true, SecondEnded: true}
	const latest, earliest mail.UnixMilli = 5_000, 1_000
	cases := []struct {
		name             string
		state            due.State
		latest, earliest mail.UnixMilli
		want             mail.UnixMilli
	}{
		{"a pass not ended with a success recorded", due.State{FirstEnded: true}, latest, earliest, latest},
		{"a first pass with runs and no success", due.State{}, 0, earliest, earliest},
		{"a new account with no run", due.State{}, 0, 0, 0},
		{"both passes ended", ended, latest, earliest, 0},
		{"the second pass marked to start over", due.State{FirstEnded: true, SecondEnded: true, Restart: true}, latest, earliest, latest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := due.Seed(c.state, c.latest, c.earliest); got != c.want {
				t.Fatalf("Seed = %d, want %d", got, c.want)
			}
		})
	}
}
