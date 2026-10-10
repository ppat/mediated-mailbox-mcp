package pass1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
)

// How a run starts, from whether the pass has ended, whether it is due again because a scanner other
// than the one in force decided what the index holds, and the latest run recorded (ADR-0017,
// ADR-0120).
func TestBegin(t *testing.T) {
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 7, Token: "p8"}, Counters: pass1.Counters{Pages: 7, Messages: 690}}
	succeeded := pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Succeeded, Progress: at}
	running := pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Running, Progress: at}
	cases := []struct {
		name       string
		ended, due bool
		latest     pass1.Latest[pass1.Progress]
		want       pass1.Start[pass1.Progress]
	}{
		{"a pass that ended", true, false, succeeded, pass1.Start[pass1.Progress]{Skip: true}},
		{"a pass that ended with a run left running", true, false, running, pass1.Start[pass1.Progress]{Skip: true}},
		{"no run yet", false, false, pass1.Latest[pass1.Progress]{}, pass1.Start[pass1.Progress]{}},
		{"no run yet, with work another scanner decided", false, true, pass1.Latest[pass1.Progress]{}, pass1.Start[pass1.Progress]{}},
		{"a run that failed", false, false, pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Failed, Progress: at}, pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: at}},
		{"a run that stopped without recording its end", false, false, running, pass1.Start[pass1.Progress]{ResumedFrom: "r1", Abandon: true, From: at}},
		{"a pass asked to run again after it succeeded", false, false, succeeded, pass1.Start[pass1.Progress]{}},
		{"a pass that ended and is due again", true, true, succeeded, pass1.Start[pass1.Progress]{Reopen: true}},
		{"a pass that ended and is due again with a run left running", true, true, running, pass1.Start[pass1.Progress]{Reopen: true, ResumedFrom: "r1", Abandon: true, From: at}},
		{"a pass not ended that is due again", false, true, running, pass1.Start[pass1.Progress]{ResumedFrom: "r1", Abandon: true, From: at}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, pass1.Begin(c.ended, c.due, c.latest), compare.Options); diff != "" {
				t.Errorf("Begin (-want +got):\n%s", diff)
			}
		})
	}
}

// A page made durable moves the checkpoint one page on to the provider's next token, keeps the scanner
// the enumeration masks under, and adds what it added and masked again to the counters. A pass started
// over keeps its counters and its scanner and returns to the first page.
func TestAdvanceAndRestart(t *testing.T) {
	stamp := index.Stamp{Version: 3, Revision: "r"}
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 2, Token: "p3", Stamp: stamp}, Counters: pass1.Counters{Pages: 4, Messages: 150, Remasked: 9, Refetched: 2}}
	want := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 3, Token: "p4", Stamp: stamp}, Counters: pass1.Counters{Pages: 5, Messages: 170, Remasked: 12, Refetched: 2}}
	if diff := cmp.Diff(want, pass1.Advance(at, "p4", nil, 20, 3), compare.Options); diff != "" {
		t.Errorf("Advance (-want +got):\n%s", diff)
	}
	last := pass1.Advance(at, "", nil, 0, 0)
	if !last.Checkpoint.Ended() || at.Checkpoint.Ended() || (pass1.Checkpoint{}).Ended() {
		t.Errorf("Ended is %v after the last page, %v mid-way and %v before the first, want true, false, false",
			last.Checkpoint.Ended(), at.Checkpoint.Ended(), (pass1.Checkpoint{}).Ended())
	}
	if diff := cmp.Diff(pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: stamp}, Counters: at.Counters}, pass1.Restart(at), compare.Options); diff != "" {
		t.Errorf("Restart (-want +got):\n%s", diff)
	}
}

// The pages an enumeration takes come from the total and page limit the page reports and its next
// token alone. A page with no total gives no estimate. Before the last page it is the items over the
// limit, rounded up, so a short last page is still a page, and never fewer than one past the current
// page while a next token says another follows. Each page's total gives a fresh figure, so a mailbox
// that grows or shrinks moves it. At the last page it is that page.
func TestAdvanceEstimatesThePagesFromTheTotal(t *testing.T) {
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 2, Token: "p3", Of: 9}}
	total := func(items, limit int) *mail.Total { return &mail.Total{Items: items, PageLimit: limit} }
	cases := []struct {
		name  string
		next  mail.PageToken
		total *mail.Total
		want  int
	}{
		{"no total", "p4", nil, 0},
		{"no total at the last page", "", nil, 0},
		{"a total filling its pages", "p4", total(30, 3), 10},
		{"a total whose last page is short", "p4", total(31, 3), 11},
		{"a mailbox that shrank below the pages taken", "p4", total(4, 3), 4},
		{"a total of nothing while pages remain", "p4", total(0, 3), 4},
		{"the last page", "", total(31, 3), 3},
		{"a page limit of nothing", "p4", total(30, 0), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pass1.Advance(at, c.next, c.total, 0, 0).Checkpoint.Of; got != c.want {
				t.Errorf("Advance to page 3 with the next token %q and the total %+v gave of %d, want %d", c.next, c.total, got, c.want)
			}
		})
	}
}

// A run starting under a scanner the enumeration it resumes was not made under starts the enumeration
// over from the first page while the enumeration has not ended, its counters carried, so an
// enumeration that ends was made under one scanner from its first page. The estimate of the pages the
// old enumeration takes is dropped with it, and a run that resumes under the same scanner keeps its
// estimate. A pass reopened after its enumeration ended starts from that enumeration's checkpoint with
// fresh counters, so it enumerates nothing, and a run resuming a pass whose enumeration ended never
// starts it over. Every run that starts records the scanner it masks under, and one whose enumeration
// has ended the stale subjects it has to fetch again (ADR-0120).
func TestUnder(t *testing.T) {
	old, now := index.Stamp{Version: 1, Revision: "old"}, index.Stamp{Version: 1, Revision: "new"}
	counters := pass1.Counters{Pages: 4, Messages: 12, Remasked: 2, Refetched: 1}
	resumed := func(page int, s index.Stamp) pass1.Start[pass1.Progress] {
		return pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Page: page, Token: "p", Of: 9, Stamp: s}, Counters: counters}}
	}
	ended := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 9, Of: 9, Stamp: old}, Counters: counters}
	none := pass1.Latest[pass1.Progress]{}
	succeeded := pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Succeeded, Progress: ended}
	failed := pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Failed, Progress: ended}
	cases := []struct {
		name     string
		start    pass1.Start[pass1.Progress]
		latest   pass1.Latest[pass1.Progress]
		want     pass1.Start[pass1.Progress]
		wantOver bool
	}{
		{"a skipped pass", pass1.Start[pass1.Progress]{Skip: true}, succeeded, pass1.Start[pass1.Progress]{Skip: true}, false},
		{"a fresh pass", pass1.Start[pass1.Progress]{}, none, pass1.Start[pass1.Progress]{From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}}}, false},
		{"a pass reopened with no run recorded", pass1.Start[pass1.Progress]{Reopen: true}, none, pass1.Start[pass1.Progress]{Reopen: true, From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}}}, false},
		{
			"a pass reopened after its enumeration ended",
			pass1.Start[pass1.Progress]{Reopen: true},
			succeeded,
			pass1.Start[pass1.Progress]{Reopen: true, From: pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 9, Of: 9, Stamp: now, Stale: 7}}},
			false,
		},
		{
			"a pass reopened after an enumeration that did not end",
			pass1.Start[pass1.Progress]{Reopen: true},
			pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Succeeded, Progress: resumed(3, old).From},
			pass1.Start[pass1.Progress]{Reopen: true, From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}}},
			false,
		},
		{"a run resumed under the same scanner", resumed(3, now), none, resumed(3, now), false},
		{
			"a run resumed under another scanner", resumed(3, old), none,
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}, Counters: counters}},
			true,
		},
		{
			"a run resumed under another version", resumed(3, index.Stamp{Version: 2, Revision: "new"}), none,
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}, Counters: counters}},
			true,
		},
		{"a run resumed before its first page", resumed(0, old), none, pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Token: "p", Of: 9, Stamp: now}, Counters: counters}}, false},
		{
			"a run resumed after its enumeration ended under another scanner",
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: ended},
			failed,
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 9, Of: 9, Stamp: now, Stale: 7}, Counters: counters}},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, over := pass1.Under(c.start, c.latest, now, 7)
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Under (-want +got):\n%s", diff)
			}
			if over != c.wantOver {
				t.Errorf("Under reports starting over %v, want %v", over, c.wantOver)
			}
		})
	}
}

// A stored subject whose message the provider left out of its answer is masked whole under the
// scanner in force, with the one event of a whole subject, as a scanner that cannot decide masks it
// (ADR-0120, ADR-0003).
func TestUnfound(t *testing.T) {
	s := index.Stamp{Version: 1, Revision: "new"}
	got := pass1.Unfound([]pass1.Stored{{ID: "m1", Subject: "Your code is ██████"}, {ID: "m2", Subject: ""}}, s)
	want := []index.Message{
		{ID: "m1", Subject: "███████████████████", SubjectMasked: true, Masks: []index.Mask{{Rule: "mask.whole_subject"}}, Stamp: s},
		{ID: "m2", Subject: "", SubjectMasked: true, Masks: []index.Mask{{Rule: "mask.whole_subject"}}, Stamp: s},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Unfound (-want +got):\n%s", diff)
	}
}

// scanner returns the default scanner under revision.
func scanner(t *testing.T, revision string) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), revision)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A subject stored unmasked is masked again from what is stored, under the scanner in force, with an
// event for each mask, and one the scanner masks nothing of keeps its stored text byte for byte
// (ADR-0120).
func TestRemasked(t *testing.T) {
	s := scanner(t, "new")
	stamp := index.StampOf(s)
	got := pass1.Remasked([]pass1.Stored{{ID: "m1", Subject: "Your verification code is 419283"}, {ID: "m2", Subject: "  Lunch\ton Friday? Café ☕ "}}, s)
	want := []index.Message{
		{ID: "m1", Subject: "Your verification code is ██████", SubjectMasked: true, Masks: []index.Mask{{Rule: scan.RuleTriggerWindow, Tier: 1}}, Stamp: stamp},
		{ID: "m2", Subject: "  Lunch\ton Friday? Café ☕ ", Stamp: stamp},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Remasked (-want +got):\n%s", diff)
	}
}

// The subjects fetched again are masked from the subject the provider returned, not from what is
// stored, and a message the provider left out of its answer is masked whole under the scanner in
// force. Metadata for a message nobody asked for is ignored (ADR-0120).
func TestRefetch(t *testing.T) {
	s := scanner(t, "new")
	stamp := index.StampOf(s)
	stored := []pass1.Stored{{ID: "m1", Subject: "Your verification code is ██████"}, {ID: "m2", Subject: "Sign in ██████"}}
	got := []mail.MessageMetadata{{ID: "m3", Subject: "not asked for"}, {ID: "m1", Subject: "Your verification code is 419283, or 552901"}}
	remasked, gone := pass1.Refetch(stored, got, s)
	if len(remasked) != 1 || remasked[0].ID != "m1" || remasked[0].Subject != "Your verification code is ██████, or ██████" ||
		!remasked[0].SubjectMasked || len(remasked[0].Masks) != 2 || remasked[0].Stamp != stamp {
		t.Errorf("Refetch masked again %+v, want m1 masked from the provider's subject with two masks under the scanner in force", remasked)
	}
	wantGone := []index.Message{{ID: "m2", Subject: "██████████████", SubjectMasked: true, Masks: []index.Mask{{Rule: "mask.whole_subject"}}, Stamp: stamp}}
	if diff := cmp.Diff(wantGone, gone, compare.Options); diff != "" {
		t.Errorf("Refetch's gone messages (-want +got):\n%s", diff)
	}
}

// A call fetching stale subjects again made durable records the stale subjects left and adds the
// subjects it masked again to the counters, leaving the enumeration's checkpoint as it was (ADR-0120).
func TestFetched(t *testing.T) {
	stamp := index.Stamp{Version: 1, Revision: "new"}
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 9, Of: 9, Stamp: stamp, Stale: 7}, Counters: pass1.Counters{Pages: 2, Refetched: 3}}
	want := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 9, Of: 9, Stamp: stamp, Stale: 4}, Counters: pass1.Counters{Pages: 2, Refetched: 6}}
	if diff := cmp.Diff(want, pass1.Fetched(at, 3, 4), compare.Options); diff != "" {
		t.Errorf("Fetched (-want +got):\n%s", diff)
	}
}

// What a run does after a failed attempt at a page. Refusing the token the run resumed from starts the
// enumeration over once. A throttled or failed page is asked for again until the last attempt, after a
// backoff when throttled. Everything else, a refused token the run got in this run included, fails the
// run.
func TestOnFailure(t *testing.T) {
	cases := []struct {
		name string
		f    pass1.Attempt
		want pass1.Next
	}{
		{"a refused resumed token", pass1.Attempt{Class: index.Validation, Attempts: 1, ResumedToken: true}, pass1.StartOver},
		{"a refused resumed token after the pass started over", pass1.Attempt{Class: index.Validation, Attempts: 1, ResumedToken: true, Restarted: true}, pass1.Abandon},
		{"a refused token the run got in this run", pass1.Attempt{Class: index.Validation, Attempts: 1}, pass1.Abandon},
		{"a first throttle", pass1.Attempt{Class: index.Throttled, Attempts: 1}, pass1.Backoff},
		{"a throttle before the last attempt", pass1.Attempt{Class: index.Throttled, Attempts: pass1.MaxAttempts - 1}, pass1.Backoff},
		{"a throttle on the last attempt", pass1.Attempt{Class: index.Throttled, Attempts: pass1.MaxAttempts}, pass1.Abandon},
		{"a provider failure", pass1.Attempt{Class: index.ProviderError, Attempts: 2}, pass1.Retry},
		{"a provider failure on the last attempt", pass1.Attempt{Class: index.ProviderError, Attempts: pass1.MaxAttempts}, pass1.Abandon},
		{"a throttle on a resumed token", pass1.Attempt{Class: index.Throttled, Attempts: 1, ResumedToken: true}, pass1.Backoff},
		{"a refused credential", pass1.Attempt{Class: index.Authentication, Attempts: 1}, pass1.Abandon},
		{"a page that is gone", pass1.Attempt{Class: index.Gone, Attempts: 1, ResumedToken: true}, pass1.Abandon},
		{"a failure that is not the provider's", pass1.Attempt{Class: index.NotProvider, Attempts: 1, ResumedToken: true}, pass1.FailRun},
		{"an attempt nobody described", pass1.Attempt{}, pass1.FailRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pass1.OnFailure(c.f); got != c.want {
				t.Errorf("OnFailure(%+v) = %v, want %v", c.f, got, c.want)
			}
		})
	}
	if pass1.MaxAttempts != 5 {
		t.Errorf("MaxAttempts is %d, want 5", pass1.MaxAttempts)
	}
}
