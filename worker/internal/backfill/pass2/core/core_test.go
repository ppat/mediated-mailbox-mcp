package core_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	pass1 "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1/core"
	pass2 "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2/core"
	indexcore "github.com/ppat/mediated-mailbox-mcp/worker/internal/index/core"
	runs "github.com/ppat/mediated-mailbox-mcp/worker/internal/runs/core"
)

var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

const account = "personal"

// listing returns the account's policy with one rule listing each of domains.
func listing(t *testing.T, domains ...string) classify.Policy {
	t.Helper()
	var rows []policy.Row
	for _, d := range domains {
		rows = append(rows, policy.Row{ID: "rule." + d, Class: policy.Restricted, DomainSuffixes: []string{d}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return classify.Prepare(s.For(account), lookups)
}

func scanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(config.DefaultScanner(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A run starts only once the first pass has ended, and then as the first pass's runs start (ADR-0133).
func TestBegin(t *testing.T) {
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: pass2.Position{SentAt: 1_700_000_030_000, ID: "m30"}}, Counters: pass2.Counters{Pages: 3, Counts: indexcore.Counts{Decided: 30, Scanned: 20, Skipped: 10}}}
	stopped := runs.Latest[pass2.Progress]{Found: true, RunID: "r1", State: runs.Failed, Progress: at}
	cases := []struct {
		name              string
		firstEnded, ended bool
		latest            runs.Latest[pass2.Progress]
		want              runs.Start[pass2.Progress]
	}{
		{"the first pass has not ended", false, false, runs.Latest[pass2.Progress]{}, runs.Start[pass2.Progress]{Skip: true}},
		{"the first pass has not ended and a run stopped", false, false, stopped, runs.Start[pass2.Progress]{Skip: true}},
		{"no run yet", true, false, runs.Latest[pass2.Progress]{}, runs.Start[pass2.Progress]{}},
		{"a run that failed", true, false, stopped, runs.Start[pass2.Progress]{ResumedFrom: "r1", From: at}},
		{"the pass ended", true, true, stopped, runs.Start[pass2.Progress]{Skip: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, pass2.Begin(c.firstEnded, c.ended, c.latest), compare.Options); diff != "" {
				t.Errorf("Begin (-want +got):\n%s", diff)
			}
		})
	}
}

// A run resuming a stopped pass while a backfill run's start marked the pass to start over starts over
// from the first waiting message, its counters carried, since what that step returned to pending may
// sit before the checkpoint. Without the mark it resumes where it stopped (ADR-0120).
func TestOver(t *testing.T) {
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: pass2.Position{SentAt: 1_700_000_030_000, ID: "m30"}}, Counters: pass2.Counters{Pages: 3, Counts: indexcore.Counts{Decided: 30, Scanned: 20, Skipped: 10}}}
	resumed := runs.Start[pass2.Progress]{ResumedFrom: "r1", From: at}
	cases := []struct {
		name     string
		start    runs.Start[pass2.Progress]
		marked   bool
		want     runs.Start[pass2.Progress]
		wantOver bool
	}{
		{"a skipped pass", runs.Start[pass2.Progress]{Skip: true}, true, runs.Start[pass2.Progress]{Skip: true}, false},
		{"a fresh pass", runs.Start[pass2.Progress]{}, true, runs.Start[pass2.Progress]{}, false},
		{"a resumed run without the mark", resumed, false, resumed, false},
		{"a resumed run with the mark", resumed, true, runs.Start[pass2.Progress]{ResumedFrom: "r1", From: pass2.Progress{Counters: at.Counters}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, over := pass2.Over(c.start, c.marked)
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Over (-want +got):\n%s", diff)
			}
			if over != c.wantOver {
				t.Errorf("Over reports starting over %v, want %v", over, c.wantOver)
			}
		})
	}
}

// Starting over keeps the counters and returns to the first waiting message, the zero position.
func TestRestart(t *testing.T) {
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: pass2.Position{SentAt: 1_700_000_030_000, ID: "m30"}}, Counters: pass2.Counters{Pages: 3, Counts: indexcore.Counts{Decided: 30}}}
	want := pass2.Progress{Counters: pass2.Counters{Pages: 3, Counts: indexcore.Counts{Decided: 30}}}
	if diff := cmp.Diff(want, pass2.Restart(at), compare.Options); diff != "" {
		t.Errorf("Restart (-want +got):\n%s", diff)
	}
}

// skipped is a message the high-volume rule skips under the thresholds below, with a List-Id, a
// sender above the high-volume mark with no prior hit, and nothing else asking for a scan.
func skipped() index.Waiting {
	return index.Waiting{
		ID: "m1", From: "news@list.example", Domain: "list.example", ListID: true,
		SizeBytes: 50_000, SentAt: 0, SenderVolume: 10, SenderHits: 0,
	}
}

var thresholds = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 30_720, RecentAgeMillis: 86_400_000, LowVolume: 2, HighVolume: 5}

const now = mail.UnixMilli(10 * 86_400_000)

// What a run does after an attempt at a body fails. A throttle backs off and then stops the run, a
// provider error retries and then abandons the message, a message the provider no longer has is
// gone, a refused request is abandoned, a body the port refused is refused at its first attempt
// (ADR-0132), a refused credential stops the run, and a failure that is not the provider's fails it.
// A decision nobody made stops the run.
func TestOnFailure(t *testing.T) {
	cases := []struct {
		name string
		in   pass2.Attempt
		want pass2.Next
	}{
		{"not the provider's", pass2.Attempt{Class: index.NotProvider, Attempts: 1}, pass2.FailRun},
		{"a first throttle", pass2.Attempt{Class: index.Throttled, Attempts: 1}, pass2.Backoff},
		{"the last throttle", pass2.Attempt{Class: index.Throttled, Attempts: pass1.MaxAttempts}, pass2.Stop},
		{"a first provider error", pass2.Attempt{Class: index.ProviderError, Attempts: 1}, pass2.Retry},
		{"the last provider error", pass2.Attempt{Class: index.ProviderError, Attempts: pass1.MaxAttempts}, pass2.Abandon},
		{"gone", pass2.Attempt{Class: index.Gone, Attempts: 1}, pass2.Gone},
		{"refused as malformed", pass2.Attempt{Class: index.Validation, Attempts: 1}, pass2.Abandon},
		{"a body the port refused", pass2.Attempt{Class: index.Refused, Attempts: 1}, pass2.Refuse},
		{"a refused credential", pass2.Attempt{Class: index.Authentication, Attempts: 1}, pass2.Stop},
		{"an unknown class", pass2.Attempt{Class: "something else", Attempts: 1}, pass2.Stop},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pass2.OnFailure(c.in); got != c.want {
				t.Errorf("OnFailure(%+v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
	if pass2.Next(0) != pass2.Stop {
		t.Errorf("the zero Next is %d, want Stop", pass2.Next(0))
	}
	if index.ClassOf(errors.New("the limiter failed")) != index.NotProvider {
		t.Errorf("a failure that is not the provider's is classed as the provider's")
	}
}

// A page advances the checkpoint to its last message's position, its send time and identifier
// (ADR-0126), and counts each message once, as decided and scanned, decided and skipped, decided and
// refused (ADR-0132), or left waiting, and its hits by sender (ADR-0016).
func TestAdvance(t *testing.T) {
	s := scanner(t)
	flagged := index.Scan(s, "Your verification code is 419283.")
	clean := index.Scan(s, "Hello there.")
	scanned := index.Gate(listing(t), thresholds, now, index.Waiting{From: "a@a.example", SenderVolume: 1}, 0)
	skip := index.Gate(listing(t), thresholds, now, skipped(), 0)
	page := pass2.Page{Last: pass2.Position{SentAt: 1_699_999_994_000, ID: "m6"}, Outcomes: []index.Outcome{
		{ID: "m1", Domain: "a.example", Verdict: scanned, Scanned: &flagged},
		{ID: "m2", Domain: "a.example", Verdict: scanned, Scanned: &clean},
		{ID: "m3", Domain: "list.example", Verdict: skip},
		{ID: "m4", Domain: "a.example", Verdict: scanned},
		{ID: "m5", Domain: "b.example", Verdict: index.Gate(listing(t), scangate.Config{}, now, skipped(), 0)},
		{ID: "m6", Domain: "a.example", Verdict: scanned, Refused: true},
	}}
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 1, After: pass2.Position{SentAt: 1_700_000_000_000, ID: "m0"}}, Counters: pass2.Counters{Pages: 1, Counts: indexcore.Counts{Decided: 1, Scanned: 1}}}
	want := pass2.Progress{
		Checkpoint: pass2.Checkpoint{Page: 2, After: pass2.Position{SentAt: 1_699_999_994_000, ID: "m6"}},
		Counters:   pass2.Counters{Pages: 2, Counts: indexcore.Counts{Decided: 6, Scanned: 3, Skipped: 1, Refused: 1, Pending: 2}},
	}
	if diff := cmp.Diff(want, pass2.Advance(at, page), compare.Options); diff != "" {
		t.Errorf("Advance (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"a.example": 1}, page.Hits(), compare.Options); diff != "" {
		t.Errorf("Hits (-want +got):\n%s", diff)
	}
}
