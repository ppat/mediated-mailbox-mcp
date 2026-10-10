package pass2_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2"
)

var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

const account = "personal"

// listing returns the account's policy with one rule listing each of domains.
func listing(t *testing.T, domains ...string) policy.Composed {
	t.Helper()
	var rows []policy.Row
	for _, d := range domains {
		rows = append(rows, policy.Row{ID: "rule." + d, Class: policy.Restricted, DomainSuffixes: []string{d}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return s.For(account)
}

func scanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A run starts only once the first pass has ended, and then as the first pass's runs start (ADR-0017).
func TestBegin(t *testing.T) {
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: "m30"}, Counters: pass2.Counters{Pages: 3, Decided: 30, Scanned: 20, Skipped: 10}}
	stopped := pass1.Latest[pass2.Progress]{Found: true, RunID: "r1", State: pass1.Failed, Progress: at}
	cases := []struct {
		name              string
		firstEnded, ended bool
		latest            pass1.Latest[pass2.Progress]
		want              pass1.Start[pass2.Progress]
	}{
		{"the first pass has not ended", false, false, pass1.Latest[pass2.Progress]{}, pass1.Start[pass2.Progress]{Skip: true}},
		{"the first pass has not ended and a run stopped", false, false, stopped, pass1.Start[pass2.Progress]{Skip: true}},
		{"no run yet", true, false, pass1.Latest[pass2.Progress]{}, pass1.Start[pass2.Progress]{}},
		{"a run that failed", true, false, stopped, pass1.Start[pass2.Progress]{ResumedFrom: "r1", From: at}},
		{"the pass ended", true, true, stopped, pass1.Start[pass2.Progress]{Skip: true}},
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
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: "m30"}, Counters: pass2.Counters{Pages: 3, Decided: 30, Scanned: 20, Skipped: 10}}
	resumed := pass1.Start[pass2.Progress]{ResumedFrom: "r1", From: at}
	cases := []struct {
		name     string
		start    pass1.Start[pass2.Progress]
		marked   bool
		want     pass1.Start[pass2.Progress]
		wantOver bool
	}{
		{"a skipped pass", pass1.Start[pass2.Progress]{Skip: true}, true, pass1.Start[pass2.Progress]{Skip: true}, false},
		{"a fresh pass", pass1.Start[pass2.Progress]{}, true, pass1.Start[pass2.Progress]{}, false},
		{"a resumed run without the mark", resumed, false, resumed, false},
		{"a resumed run with the mark", resumed, true, pass1.Start[pass2.Progress]{ResumedFrom: "r1", From: pass2.Progress{Counters: at.Counters}}, true},
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

// Starting over keeps the counters and returns to the first waiting message.
func TestRestart(t *testing.T) {
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 3, After: "m30"}, Counters: pass2.Counters{Pages: 3, Decided: 30}}
	want := pass2.Progress{Counters: pass2.Counters{Pages: 3, Decided: 30}}
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

// A stored gate skip stands only while the gate, deciding it again under the thresholds given, decides
// it as the same skip. A skip the gate would now scan, every skip under thresholds that cannot decide
// and a skip whose sender the policy now restricts are overturned (ADR-0121, ADR-0093).
func TestOverturned(t *testing.T) {
	skip := func(id string, volume int64) index.Waiting {
		m := skipped()
		m.ID, m.SenderVolume = id, volume
		return m
	}
	hit, masked, bank := skip("c", 10), skip("d", 10), skip("e", 30)
	hit.SenderHits = 1
	masked.SubjectMasked = true
	bank.From, bank.Domain = "news@bank.example", "bank.example"
	skips := []index.Waiting{skip("a", 10), skip("b", 30), hit, masked, bank}
	widened := thresholds
	widened.HighVolume = 20
	cases := []struct {
		name       string
		policy     policy.Composed
		thresholds scangate.Config
		skips      []index.Waiting
		want       []string
	}{
		{"the thresholds the skips were made under", listing(t), thresholds, skips, []string{"c", "d"}},
		{"a widened high-volume mark", listing(t), widened, skips, []string{"a", "c", "d"}},
		{"thresholds that cannot decide", listing(t), scangate.Config{}, skips, []string{"a", "b", "c", "d", "e"}},
		{"a sender the policy now lists", listing(t, "bank.example"), thresholds, skips, []string{"c", "d", "e"}},
		{"no skips", listing(t), thresholds, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, pass2.Overturned(c.policy, lookups, c.thresholds, now, c.skips), compare.Options); diff != "" {
				t.Errorf("Overturned (-want +got):\n%s", diff)
			}
		})
	}
}

// What a run does after an attempt at a body fails. A throttle backs off and then stops the run, a
// provider error retries and then abandons the message, a message the provider no longer has is
// gone, a refused request is abandoned, a refused credential stops the run, and a failure that is not
// the provider's fails it. A decision nobody made stops the run.
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

// A page advances the checkpoint to its last message and counts each message once, as decided and
// scanned, decided and skipped, or left waiting, and its hits by sender (ADR-0016).
func TestAdvance(t *testing.T) {
	s := scanner(t)
	flagged := index.Scan(s, "Your verification code is 419283.", "")
	clean := index.Scan(s, "Hello there.", "")
	scanned := index.Gate(listing(t), lookups, thresholds, now, index.Waiting{From: "a@a.example", SenderVolume: 1}, 0)
	skip := index.Gate(listing(t), lookups, thresholds, now, skipped(), 0)
	page := pass2.Page{Last: "m5", Outcomes: []index.Outcome{
		{ID: "m1", Domain: "a.example", Verdict: scanned, Scanned: &flagged},
		{ID: "m2", Domain: "a.example", Verdict: scanned, Scanned: &clean},
		{ID: "m3", Domain: "list.example", Verdict: skip},
		{ID: "m4", Domain: "a.example", Verdict: scanned},
		{ID: "m5", Domain: "b.example", Verdict: index.Gate(listing(t), lookups, scangate.Config{}, now, skipped(), 0)},
	}}
	at := pass2.Progress{Checkpoint: pass2.Checkpoint{Page: 1, After: "m0"}, Counters: pass2.Counters{Pages: 1, Decided: 1, Scanned: 1}}
	want := pass2.Progress{
		Checkpoint: pass2.Checkpoint{Page: 2, After: "m5"},
		Counters:   pass2.Counters{Pages: 2, Decided: 5, Scanned: 3, Skipped: 1, Pending: 2},
	}
	if diff := cmp.Diff(want, pass2.Advance(at, page), compare.Options); diff != "" {
		t.Errorf("Advance (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"a.example": 1}, page.Hits(), compare.Options); diff != "" {
		t.Errorf("Hits (-want +got):\n%s", diff)
	}
}
