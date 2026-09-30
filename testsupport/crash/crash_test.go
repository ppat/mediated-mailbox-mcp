package crash_test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/crash"
)

// counter is a toy machinery. A write adds one to what the process holds, a flush makes it durable,
// and a crash loses what the process held. The store is the durable count.
type counter struct {
	start, held, durable, flushed int
	log                           []string
	// forgets makes recovery lose the durable count after two crashes in a row, a sequence-dependent
	// fault planted for the harness to find. broken is a fault planted in the stand-in for PostgreSQL
	// alone, which every sequence reaches and whose report names the setup the world was built from.
	forgets, broken bool
	lastCrash       bool
}

// toy returns the counter's target. A fault is planted in the model's world, in the stand-in for
// PostgreSQL's world, or in neither.
func toy(forgetfulModel, brokenStore bool) crash.Target[int, *counter] {
	return crash.Target[int, *counter]{
		Setup: func(t *rapid.T) int { return rapid.IntRange(0, 3).Draw(t, "start") },
		Model: func(_ rapid.TB, start int) *counter {
			return &counter{start: start, held: start, durable: start, flushed: start, forgets: forgetfulModel}
		},
		Real: func(_ testing.TB, start int) *counter {
			return &counter{start: start, held: start, durable: start, flushed: start, broken: brokenStore}
		},
		Operations: map[string]crash.Operation[*counter]{
			"write": {Apply: func(_ rapid.TB, c *counter, _ int) {
				c.held++
				c.lastCrash = false
				c.log = append(c.log, "write")
			}},
			"flush": {Apply: func(_ rapid.TB, c *counter, _ int) {
				c.durable, c.flushed = c.held, c.held
				c.lastCrash = false
				c.log = append(c.log, "flush")
			}},
		},
		CrashAt: rapid.IntRange(0, 1),
		Crash: func(_ rapid.TB, c *counter, at int) {
			if c.forgets && c.lastCrash {
				c.durable = 0
			}
			c.lastCrash = true
			c.log = append(c.log, fmt.Sprintf("crash %d", at))
		},
		Recover: func(_ rapid.TB, c *counter) {
			c.held = c.durable
			c.log = append(c.log, "recover")
		},
		Persistence: func(t rapid.TB, c *counter) {
			c.log = append(c.log, "persistence")
			if c.durable < c.flushed {
				t.Errorf("the durable count is %d after a crash, below the %d flushed", c.durable, c.flushed)
			}
		},
		// Forward progress flushes what the process holds and checks it became durable. It cannot
		// see a durable count lost to a crash, which only the persistence check sees.
		Progress: func(t rapid.TB, c *counter) {
			c.log = append(c.log, "progress")
			if c.broken {
				t.Errorf("the store built from setup %d is broken", c.start)
			}
			c.durable, c.flushed = c.held, c.held
			if c.durable != c.held {
				t.Errorf("the durable count is %d after a flush of %d", c.durable, c.held)
			}
		},
	}
}

// A sequence runs its operations in order. After each crash the harness runs recovery and checks
// persistence, and at the end of the sequence it checks forward progress (ADR-0045).
func TestRunRecoversAndChecksAfterEachCrash(t *testing.T) {
	target := toy(false, false)
	w := target.Model(t, 0)
	crash.Run(t, target, w, crash.Case[int]{Ops: []crash.Op{{Name: "write"}, {Name: crash.Crash, Arg: 1}, {Name: "flush"}, {Name: crash.Crash}}})
	want := []string{"write", "crash 1", "recover", "persistence", "flush", "crash 0", "recover", "persistence", "progress"}
	if diff := cmp.Diff(want, w.log, compare.Options); diff != "" {
		t.Errorf("the calls (-want +got):\n%s", diff)
	}
}

// A report describes the operation sequence and the setup, and never the weights it was drawn under,
// which reduction flattens and which would contradict the sequence (ADR-0069, row 14).
func TestDescribeNamesTheOperations(t *testing.T) {
	got := crash.Describe(crash.Case[int]{Setup: 2, Ops: []crash.Op{{Name: "write"}, {Name: crash.Crash, Arg: 1}}})
	if want := "setup 2, operations [write(0) crash(1)]"; got != want {
		t.Errorf("Describe returned %q, want %q", got, want)
	}
}

// Both draws place the crash among the target's own operations, with its crash point drawn, and draw
// nothing else.
func TestBothDrawsPlaceTheCrash(t *testing.T) {
	for name, draw := range crash.Draws(toy(false, false), crash.Config{MaxOps: 20}) {
		t.Run(name, func(t *testing.T) {
			gen := rapid.Custom(draw)
			seen := map[string]bool{}
			for seed := range 200 {
				for _, op := range gen.Example(seed).Ops {
					seen[fmt.Sprintf("%s(%d)", op.Name, op.Arg)] = true
				}
			}
			want := []string{"crash(0)", "crash(1)", "flush(0)", "write(0)"}
			got := make([]string, 0, len(seen))
			for k := range seen {
				got = append(got, k)
			}
			slices.Sort(got)
			if diff := cmp.Diff(want, got, compare.Options); diff != "" {
				t.Errorf("the operations drawn over 200 sequences (-want +got):\n%s", diff)
			}
		})
	}
}

// plantedEnv makes TestPlanted run, planting the fault where its value says, model or store. It is set
// only by the tests below, which run this test binary again.
const plantedEnv = "CRASH_HARNESS_PLANTED"

// replays is how many fixed-seed sequences TestPlanted replays against the stand-in for PostgreSQL.
const replays = 8

// TestPlanted runs the harness over the counter with a fault that only two crashes in a row reach,
// planted in the model or in the stand-in for PostgreSQL. It runs only inside the tests below.
func TestPlanted(t *testing.T) {
	where := os.Getenv(plantedEnv)
	if where == "" {
		t.Skip("run by TestTheHarnessFindsAPlantedFault and TestTheReplaysFindAFaultOnlyPostgreSQLHas")
	}
	crash.Check(t, toy(where == "model", where == "store"), crash.Config{MaxOps: 30, Replays: replays})
}

// planted runs TestPlanted with the fault planted where says, and returns its output.
func planted(t *testing.T, where string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPlanted$", "-test.v") //nolint:gosec // The test binary runs itself.
	cmd.Env = append(os.Environ(), plantedEnv+"="+where, "RAPID_CHECKS=200")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the harness passed a fault planted in the %s:\n%s", where, out)
	}
	for _, draw := range []string{"rapid-draw", "sampled-mix"} {
		if !strings.Contains(string(out), "--- FAIL: TestPlanted/"+draw) {
			t.Errorf("the %s draw did not find the fault planted in the %s:\n%s", draw, where, out)
		}
	}
	return string(out)
}

// The harness goes red on a fault that needs two crashes in a row, in both draws, reduces the failing
// sequence to one holding both crashes, and says the model differs from the store it stands for when
// the replay against that store passes (ADR-0045, ADR-0069).
func TestTheHarnessFindsAPlantedFault(t *testing.T) {
	out := planted(t, "model")
	if strings.Contains(out, "fails against PostgreSQL and") {
		t.Errorf("a fault the search found against the model was reported as a replay's:\n%s", out)
	}
	reduced := regexp.MustCompile(`the reduced sequence fails against the model and not against PostgreSQL, so the model differs from the store it stands for: setup \d+, operations \[([^\]]*)\]`)
	matches := reduced.FindAllStringSubmatch(out, -1)
	if len(matches) != 2 {
		t.Fatalf("found %d reports of a reduced sequence the replay passed, want one per draw:\n%s", len(matches), out)
	}
	for _, m := range matches {
		if n := strings.Count(m[1], "crash("); n < 2 {
			t.Errorf("the reduced sequence [%s] holds %d crashes, and the fault needs two", m[1], n)
		}
	}
}

// The fixed-seed replays find a fault only PostgreSQL has, which the search against the model cannot
// see, in both draws. Each draw replays exactly its fixed number of sequences, from seeds 0 upward,
// each a different case, each against a world built from that case's own setup. Every failure says
// a replay failed against PostgreSQL and passed against the model, and never that the search's
// reduced sequence showed the model to differ, since the search passed (ADR-0069).
func TestTheReplaysFindAFaultOnlyPostgreSQLHas(t *testing.T) {
	out := planted(t, "store")
	if strings.Contains(out, "reduced sequence") {
		t.Errorf("a replay's failure was reported as the search's reduced sequence:\n%s", out)
	}
	replay := regexp.MustCompile(`the sequence from seed (\d+) fails against PostgreSQL and passes against the model, so PostgreSQL has a fault the model lacks: setup (\d+), operations \[([^\]]*)\]\s*\n\s*the store built from setup (\d+) is broken`)
	for _, draw := range []string{"rapid-draw", "sampled-mix"} {
		start := strings.Index(out, "=== RUN   TestPlanted/"+draw)
		if start < 0 {
			t.Fatalf("no output for the %s draw:\n%s", draw, out)
		}
		section := out[start+1:]
		if end := strings.Index(section, "=== RUN"); end >= 0 {
			section = section[:end]
		}
		var seeds []string
		cases := map[string]bool{}
		for _, m := range replay.FindAllStringSubmatch(section, -1) {
			seeds = append(seeds, m[1])
			cases[m[2]+" "+m[3]] = true
			if m[2] != m[4] {
				t.Errorf("%s: the sequence from seed %s has setup %s and was replayed against a world built from setup %s", draw, m[1], m[2], m[4])
			}
		}
		want := make([]string, replays)
		for i := range want {
			want[i] = fmt.Sprint(i)
		}
		if diff := cmp.Diff(want, seeds, compare.Options); diff != "" {
			t.Errorf("%s: the seeds of the sequences replayed against PostgreSQL (-want +got):\n%s\n%s", draw, diff, section)
		}
		if len(cases) < 2 {
			t.Errorf("%s: the %d replays replayed %d distinct case, so they are not drawn from distinct seeds:\n%s", draw, len(seeds), len(cases), section)
		}
	}
}
