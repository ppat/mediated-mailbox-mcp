package pass2_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
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
// sit before the checkpoint. Without the mark it resumes where it stopped (ADR-0096).
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

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, its comparison. A
// domain stored as restricted or skipped as restricted that the policy no longer lists is delisted,
// one still listed or listed through a parent domain is not, a domain the classifier cannot read
// stays restricted, and a policy that never loaded delists nothing (ADR-0037).
func TestDelisted(t *testing.T) {
	stored := []string{"bank.example", "mail.gov.example", "shop.example", ""}
	cases := []struct {
		name   string
		policy policy.Composed
		want   []string
	}{
		{"every rule in place", listing(t, "bank.example", "gov.example", "shop.example"), nil},
		{"the bank's rule removed", listing(t, "gov.example", "shop.example"), []string{"bank.example"}},
		{"every rule removed", listing(t), []string{"bank.example", "mail.gov.example", "shop.example"}},
		{"a policy that never loaded", policy.Composed{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, pass2.Delisted(c.policy, lookups, stored), compare.Options); diff != "" {
				t.Errorf("Delisted (-want +got):\n%s", diff)
			}
		})
	}
}

// skipped is a message the high-volume rule skips under the thresholds below: a List-Id, a sender
// above the high-volume mark with no prior hit, and nothing else asking for a scan.
func skipped() pass2.Message {
	return pass2.Message{
		ID: "m1", From: "news@list.example", Domain: "list.example", ListID: true,
		SizeBytes: 50_000, SentAt: 0, SenderVolume: 10, SenderHits: 0,
	}
}

var thresholds = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 30_720, RecentAgeMillis: 86_400_000, LowVolume: 2, HighVolume: 5}

const now = mail.UnixMilli(10 * 86_400_000)

// The gate decides from the message's own inputs, its sender's class under the policy in force and
// the sender's prior hits counting the hits found earlier on the same page, so a sender's first hit
// reaches its next message (ADR-0093, ADR-0094). A restricted sender is skipped as restricted whatever
// else asks for a scan (ADR-0008).
func TestGate(t *testing.T) {
	cases := []struct {
		name     string
		policy   policy.Composed
		message  func(pass2.Message) pass2.Message
		pageHits int64
		want     string
	}{
		{"the high-volume skip", listing(t), func(m pass2.Message) pass2.Message { return m }, 0, "high_volume_no_hits"},
		{"a hit earlier on the page", listing(t), func(m pass2.Message) pass2.Message { return m }, 1, "prior_hit"},
		{"a stored prior hit", listing(t), func(m pass2.Message) pass2.Message { m.SenderHits = 1; return m }, 0, "prior_hit"},
		{"no List-Id", listing(t), func(m pass2.Message) pass2.Message { m.ListID = false; return m }, 0, "default"},
		{"a masked subject", listing(t), func(m pass2.Message) pass2.Message { m.SubjectMasked = true; return m }, 0, "subject_signal"},
		{
			"a listed local part without a List-Id", listing(t),
			func(m pass2.Message) pass2.Message { m.ListID, m.From = false, "NoReply@list.example"; return m }, 0, "noreply_local_part",
		},
		{"a sender the policy lists", listing(t, "list.example"), func(m pass2.Message) pass2.Message { m.SubjectMasked = true; return m }, 1, "restricted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pass2.Gate(c.policy, lookups, thresholds, now, c.message(skipped()), c.pageHits)
			if got.Reason().String() != c.want {
				t.Errorf("Gate decided %q, want %q", got.Reason().String(), c.want)
			}
		})
	}
}

// A scanned body records a flag found in either part, the content rules that fired in either, each
// once and sorted, and the scanner's version and revision. A scanner nobody built flags the body
// (ADR-0009, ADR-0017).
func TestScan(t *testing.T) {
	const code, link = "Your verification code is 419283.", "Sign in: https://x.example/login/h3J9dK2mQ8xR5tY1vB7nW4sZ6"
	type got struct {
		MFA, Link bool
		Rules     []string
		Version   int
		Revision  string
	}
	observe := func(s pass2.Scanned) got {
		return got{MFA: s.Flags.MFACode(), Link: s.Flags.LoginLink(), Rules: s.Rules, Version: s.Version, Revision: s.Revision}
	}
	s := scanner(t)
	cases := []struct {
		name           string
		scanner        scan.Scanner
		markdown, text string
		want           got
	}{
		{"a clean body", s, "Hello there.", "Hello there.", got{Rules: []string{}, Version: 1, Revision: "a-revision"}},
		{"a code in the HTML part", s, code, "Hello there.", got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a code in the text part", s, "Hello there.", code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a code in both parts", s, code, code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{
			"a code in one part and a link in the other", s, link, code,
			got{MFA: true, Link: true, Rules: []string{scan.RuleLinkPath, scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"},
		},
		{"a body with no HTML part", s, "", code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a scanner nobody built", scan.Scanner{}, "Hello there.", "Hello there.", got{MFA: true, Link: true, Rules: []string{}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(pass2.Scan(c.scanner, c.markdown, c.text)), compare.Options); diff != "" {
				t.Errorf("Scan (-want +got):\n%s", diff)
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
		{"not the provider's", pass2.Attempt{Class: pass1.NotProvider, Attempts: 1}, pass2.FailRun},
		{"a first throttle", pass2.Attempt{Class: pass1.Throttled, Attempts: 1}, pass2.Backoff},
		{"the last throttle", pass2.Attempt{Class: pass1.Throttled, Attempts: pass1.MaxAttempts}, pass2.Stop},
		{"a first provider error", pass2.Attempt{Class: pass1.ProviderError, Attempts: 1}, pass2.Retry},
		{"the last provider error", pass2.Attempt{Class: pass1.ProviderError, Attempts: pass1.MaxAttempts}, pass2.Abandon},
		{"gone", pass2.Attempt{Class: pass1.Gone, Attempts: 1}, pass2.Gone},
		{"refused as malformed", pass2.Attempt{Class: pass1.Validation, Attempts: 1}, pass2.Abandon},
		{"a refused credential", pass2.Attempt{Class: pass1.Authentication, Attempts: 1}, pass2.Stop},
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
	if pass1.ClassOf(errors.New("the limiter failed")) != pass1.NotProvider {
		t.Errorf("a failure that is not the provider's is classed as the provider's")
	}
}

// A page advances the checkpoint to its last message and counts each message once, as decided and
// scanned, decided and skipped, or left waiting, and its hits by sender (ADR-0016).
func TestAdvance(t *testing.T) {
	s := scanner(t)
	flagged := pass2.Scan(s, "Your verification code is 419283.", "")
	clean := pass2.Scan(s, "Hello there.", "")
	scanned := pass2.Gate(listing(t), lookups, thresholds, now, pass2.Message{From: "a@a.example", SenderVolume: 1}, 0)
	skip := pass2.Gate(listing(t), lookups, thresholds, now, skipped(), 0)
	page := pass2.Page{Last: "m5", Outcomes: []pass2.Outcome{
		{ID: "m1", Domain: "a.example", Verdict: scanned, Scanned: &flagged},
		{ID: "m2", Domain: "a.example", Verdict: scanned, Scanned: &clean},
		{ID: "m3", Domain: "list.example", Verdict: skip},
		{ID: "m4", Domain: "a.example", Verdict: scanned},
		{ID: "m5", Domain: "b.example", Verdict: pass2.Gate(listing(t), lookups, scangate.Config{}, now, skipped(), 0)},
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
