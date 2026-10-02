package pass1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// How a run starts, from whether the pass has ended, whether it is due again because a scanner other
// than the one in force decided what the index holds, and the latest run recorded (ADR-0017,
// ADR-0096).
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
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 2, Token: "p3", Stamp: stamp}, Counters: pass1.Counters{Pages: 4, Messages: 150, Remasked: 9}}
	want := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 3, Token: "p4", Stamp: stamp}, Counters: pass1.Counters{Pages: 5, Messages: 170, Remasked: 12}}
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
// over from the first page, its counters carried, so an enumeration that ends was made under one
// scanner from its first page. The estimate of the pages the old enumeration takes is dropped with
// it, and a run that resumes under the same scanner keeps its estimate. Every run that starts records
// the scanner it masks under (ADR-0096).
func TestUnder(t *testing.T) {
	old, now := index.Stamp{Version: 1, Revision: "old"}, index.Stamp{Version: 1, Revision: "new"}
	counters := pass1.Counters{Pages: 4, Messages: 12, Remasked: 2}
	resumed := func(page int, s index.Stamp) pass1.Start[pass1.Progress] {
		return pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Page: page, Token: "p", Of: 9, Stamp: s}, Counters: counters}}
	}
	cases := []struct {
		name     string
		start    pass1.Start[pass1.Progress]
		want     pass1.Start[pass1.Progress]
		wantOver bool
	}{
		{"a skipped pass", pass1.Start[pass1.Progress]{Skip: true}, pass1.Start[pass1.Progress]{Skip: true}, false},
		{"a fresh pass", pass1.Start[pass1.Progress]{Reopen: true}, pass1.Start[pass1.Progress]{Reopen: true, From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}}}, false},
		{"a run resumed under the same scanner", resumed(3, now), resumed(3, now), false},
		{
			"a run resumed under another scanner", resumed(3, old),
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}, Counters: counters}},
			true,
		},
		{
			"a run resumed under another version", resumed(3, index.Stamp{Version: 2, Revision: "new"}),
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Stamp: now}, Counters: counters}},
			true,
		},
		{"a run resumed before its first page", resumed(0, old), pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: pass1.Progress{Checkpoint: pass1.Checkpoint{Token: "p", Of: 9, Stamp: now}, Counters: counters}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, over := pass1.Under(c.start, now)
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Under (-want +got):\n%s", diff)
			}
			if over != c.wantOver {
				t.Errorf("Under reports starting over %v, want %v", over, c.wantOver)
			}
		})
	}
}

// A stored subject the enumeration did not find is masked whole under the scanner in force, with the
// one event of a whole subject, as a scanner that cannot decide masks it (ADR-0096, ADR-0003).
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
