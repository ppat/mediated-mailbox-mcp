//go:build integration

package pass2_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/crash"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
	pass1core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass1"
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2"
)

// drawSetup draws a mailbox and a page size. Each message is drawn without seeing the ones before it,
// by rapid's collection generator, so rapid reduces a mailbox by dropping messages. ADR-0069's
// rules require both, a generator of a sensitivity-carrying value drawing each field independently
// and a list built by a collection generator.
func drawSetup(t *rapid.T) setup {
	message := rapid.Custom(func(t *rapid.T) drawn {
		return drawn{
			Sender:      rapid.IntRange(0, len(senderDomains)-1).Draw(t, "sender"),
			SubjectCode: rapid.Bool().Draw(t, "subject code"),
			BodyCode:    rapid.Bool().Draw(t, "body code"),
			HTML:        rapid.Bool().Draw(t, "html"),
			ListID:      rapid.Bool().Draw(t, "list id"),
		}
	})
	return setup{
		PageSize: rapid.IntRange(1, 4).Draw(t, "page size"),
		Messages: rapid.SliceOfN(message, 0, 12).Draw(t, "messages"),
	}
}

// target is pass 2's checkpoint and resume, the crash harness's backfill target for the second pass
// (ADR-0045). A page step makes the next page durable, a throttle makes the provider throttle its next
// body fetches, so a run can stop on a message and a later run resume the page, and a delist removes
// the bank's rule from the policy the next process loads, so its recovery runs the delisting
// transition and starts over, and a rescan changes the scanner the next process scans with and stops
// the process, so the next run reopens the pass or returns the verdicts made under the earlier scanner
// to pending and starts over, or, with its argument 1, changes it back to the scanner before the last
// change, so a pass resumed under a scanner it already ran under still reads what the change returned
// to pending (ADR-0120). A widen raises the gate's high-volume mark above any sender's volume and stops
// the process, as a release changing the thresholds does, so the next run returns every gate skip to
// pending and the pass scans it (ADR-0121).
func target() crash.Target[setup, *world] {
	return crash.Target[setup, *world]{
		Setup: drawSetup,
		Model: func(t rapid.TB, s setup) *world { return modelWorld(t, s) },
		Real:  func(t testing.TB, s setup) *world { return realWorld(t, s) },
		Operations: map[string]crash.Operation[*world]{
			"page": {Apply: func(t rapid.TB, w *world, _ int) { w.step(t) }},
			"throttle": {
				Arg:   rapid.IntRange(1, 6),
				Apply: func(_ rapid.TB, w *world, n int) { w.throttle = n },
			},
			"delist": {Apply: func(_ rapid.TB, w *world, _ int) { w.delisted = true }},
			"widen":  {Apply: func(_ rapid.TB, w *world, _ int) { w.widen() }},
			"rescan": {
				Arg: rapid.IntRange(0, 1),
				Apply: func(t rapid.TB, w *world, revert int) {
					if revert == 1 {
						w.revert(t)
						return
					}
					w.rescan(t)
				},
			},
		},
		CrashAt:     rapid.IntRange(betweenSteps, afterCommit),
		Crash:       func(t rapid.TB, w *world, at int) { w.crash(t, at) },
		Recover:     func(t rapid.TB, w *world) { w.open(t) },
		Persistence: func(t rapid.TB, w *world) { w.persistence(t) },
		Progress:    func(t rapid.TB, w *world) { w.progress(t) },
	}
}

// config bounds the sampler's sequences near the length rapid's own draw averages, and replays a few
// sequences of each draw against PostgreSQL.
var config = crash.Config{Replays: 5, MaxOps: 40}

// VERIFICATIONS' row "Kill the process running backfill mid-run", D2's part, the second pass. A
// process killed between pages, inside a page once a body is fetched and before its commit ends, or
// after the commit ends and before the run records anything more, loses no decision it reported
// durable, and the next run resumes from exactly the checkpoint last reported, or starts over when
// its delisting transition returned messages to pending scan. Every sequence then ends the pass
// with no restricted sender's body fetched, every message decided and recorded once under the
// scanner in force, each sender's prior hits counted once, no message left skipped by the gate once
// its thresholds were widened past every sender's volume, and no more than a page of bodies fetched
// again for each crash and the mailbox for each change of scanner (ADR-0017, ADR-0037, ADR-0045,
// ADR-0120, ADR-0121).
//
// The model never writes the stored form of a checkpoint, so a break in it is seen only by a sequence
// run against PostgreSQL, and the replays drawn from fixed seeds reach a resume after a durable page
// only by chance. The subtest below runs that sequence against PostgreSQL in every run (ADR-0069).
func TestAKilledSecondPassResumesFromItsCheckpoint(t *testing.T) {
	crash.Check(t, target(), config)
	t.Run("resumed-after-a-page-that-left-a-message-waiting", resumedAfterAPageThatLeftAMessageWaiting)
}

// resumedAfterAPageThatLeftAMessageWaiting kills the process after its first page is durable, where
// that page left its one message waiting, a body the conversion refuses (ADR-0017). The next run
// resumes from page 1 after that message, so it never reads the message again, never asks for its
// body again and records its failed item once, and the pass ends with the two messages after it
// scanned. A run that resumed from the first waiting message instead would read the refused message
// again, which only the position the checkpoint holds tells apart.
func resumedAfterAPageThatLeftAMessageWaiting(t *testing.T) {
	refused := "<p>" + strings.Repeat("a", 512*1024-6) + "</p>"
	w := realWorldOf(t, 1, []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("refused"), refused),
		message("m02", "b@shop.example", false, marker.Body("second"), ""),
		message("m03", "c@shop.example", false, marker.Body("third"), ""),
	})
	w.step(t)
	w.crash(t, betweenSteps)
	w.open(t)
	w.persistence(t)
	got := w.inspect(t)
	if latest, _ := got.latest(); latest.Progress.Checkpoint != (core.Checkpoint{Page: 1, After: "m01"}) {
		t.Errorf("the resumed run starts from %+v, want page 1 after m01", latest.Progress.Checkpoint)
	}
	for range 10 {
		if w.step(t) {
			break
		}
	}
	got = w.inspect(t)
	if !got.Ended {
		t.Fatalf("the pass did not end")
	}
	states := map[string]string{}
	for id, m := range got.Messages {
		states[id] = m.State
	}
	if diff := cmp.Diff(map[string]string{"m01": "pending", "m02": "scanned", "m03": "scanned"}, states, compare.Options); diff != "" {
		t.Errorf("the scan states (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"m01", "m02", "m03"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string][]string{"m01": {"abandoned"}}, got.Items, compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}
}

// The generator report for both draws (ADR-0069). Each kind of sequence the checks rest on is reached,
// stated as whether it is reached at all, not as an expected share.
func TestTheSecondPassCrashSequencesReachEveryKind(t *testing.T) {
	minimums := map[string]float64{
		"a crash inside a page":                          0.001,
		"a crash between steps after a page was durable": 0.001,
		"a crash after a page's commit":                  0.001,
		"a delisting recovered after a page was durable": 0.001,
		"a run stopped on a throttled body":              0.001,
		"a change of scanner after a body was scanned":   0.001,
		"a change of scanner reverted":                   0.001,
		"a change of thresholds after a page":            0.001,
	}
	for name, draw := range crash.Draws(target(), config) {
		t.Run(name, func(t *testing.T) {
			property.Report(t, draw, kind, minimums)
		})
	}
}

// kind classifies a case by the rarest telling thing its sequence does, in the order listed. A run
// stops on a throttled body only when a step fetches a body while the provider still has at least
// pass1core.MaxAttempts throttles to give. A body is fetched on a page only when the mailbox holds a
// sender the gate scans, which the classifier approximates by a message from a normal sender.
func kind(c crash.Case[setup]) string {
	bodies := false
	for _, m := range c.Setup.Messages {
		if senderDomains[m.Sender%len(senderDomains)] != listedDomain && senderDomains[m.Sender%len(senderDomains)] != "" {
			bodies = true
		}
	}
	pages, pending := 0, 0
	delistPending, rescanned, reverted, widened := false, false, false, false
	changes := 0
	inside, afterPage, afterCommitted, delistRecovered, stoppedOnThrottle := false, false, false, false, false
	for _, op := range c.Ops {
		switch op.Name {
		case crash.Crash:
			switch {
			case op.Arg == insidePage && pages > 0:
				inside = true
			case op.Arg == afterCommit && pages > 0:
				afterCommitted = true
			case pages > 0:
				afterPage = true
			}
			if delistPending && pages > 0 {
				delistRecovered = true
			}
			delistPending = false
		case "page":
			pages++
			if bodies && pending >= pass1core.MaxAttempts {
				stoppedOnThrottle = true
			}
			pending = 0
		case "throttle":
			pending = op.Arg
		case "delist":
			delistPending = true
		case "widen":
			if pages > 0 {
				widened = true
			}
		case "rescan":
			switch {
			case op.Arg == 1 && changes > 0:
				changes--
				reverted = true
			case op.Arg == 0:
				changes++
				if bodies && pages > 0 {
					rescanned = true
				}
			}
		}
	}
	switch {
	case rescanned:
		return "a change of scanner after a body was scanned"
	case stoppedOnThrottle:
		return "a run stopped on a throttled body"
	case delistRecovered:
		return "a delisting recovered after a page was durable"
	case reverted:
		return "a change of scanner reverted"
	case afterCommitted:
		return "a crash after a page's commit"
	case afterPage:
		return "a crash between steps after a page was durable"
	case inside:
		return "a crash inside a page"
	case widened:
		return "a change of thresholds after a page"
	default:
		return "another sequence"
	}
}
