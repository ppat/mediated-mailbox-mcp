package due_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due"
)

// A tick is due at the first ask, and then once the sync interval has passed since the last tick was
// asked for, never before (ADR-0103, ADR-0119).
func TestDeltaSyncsDueDecision(t *testing.T) {
	const t0 = mail.UnixMilli(1_791_000_000_000)
	const interval = 5 * 60 * 1000
	cases := []struct {
		name     string
		ticked   bool
		last, at mail.UnixMilli
		want     bool
	}{
		{"no tick yet in this process", false, 0, t0, true},
		{"asked one interval after the last tick", true, t0, t0 + interval, true},
		{"asked after more than an interval", true, t0, t0 + 17*60*1000, true},
		{"asked a moment before the interval has passed", true, t0, t0 + interval - 1, false},
		{"asked at the end of a backoff after a failed tick", true, t0, t0 + 60*1000, false},
		{"asked at once again", true, t0, t0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := due.Decide(c.ticked, c.last, c.at, interval); got != c.want {
				t.Fatalf("Decide = %v, want %v", got, c.want)
			}
		})
	}
}

// A delta sync job counts from its latest recorded success, from its earliest tick's start while it
// has none, and from the ensure when its ticks record nothing (ADR-0119).
func TestDeltaSyncsSeed(t *testing.T) {
	cases := []struct {
		name                   string
		latest, earliest, want mail.UnixMilli
	}{
		{"a success recorded", 5_000, 1_000, 5_000},
		{"ticks and no success", 0, 1_000, 1_000},
		{"no tick", 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := due.Seed(c.latest, c.earliest); got != c.want {
				t.Fatalf("Seed = %d, want %d", got, c.want)
			}
		})
	}
}
