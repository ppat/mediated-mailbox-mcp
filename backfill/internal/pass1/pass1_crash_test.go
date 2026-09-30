//go:build integration

package pass1_test

import (
	"testing"

	"pgregory.net/rapid"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/crash"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// drawSetup draws a mailbox and a page size. Each message is drawn without seeing the ones before it,
// by rapid's collection generator, so rapid reduces a mailbox by dropping messages (ADR-0069, rows 10
// and 11 of its ordinary-path table).
func drawSetup(t *rapid.T) setup {
	message := rapid.Custom(func(t *rapid.T) drawn {
		return drawn{
			Sender: rapid.IntRange(0, len(senderDomains)-1).Draw(t, "sender"),
			Code:   rapid.Bool().Draw(t, "code"),
			Inbox:  rapid.Bool().Draw(t, "inbox"),
			ListID: rapid.Bool().Draw(t, "list id"),
		}
	})
	return setup{
		PageSize: rapid.IntRange(1, 4).Draw(t, "page size"),
		Messages: rapid.SliceOfN(message, 0, 12).Draw(t, "messages"),
	}
}

// target is pass 1's checkpoint and resume, the crash harness's backfill target (ADR-0045). A page
// step makes the next page durable, and a throttle makes the provider throttle its next calls, so a
// run can fail on a page and a later run resume it.
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

// VERIFICATIONS' row for killing the backfill pod mid-run, D1's part, the mechanism. A process killed
// between pages, inside a page before its commit ends, or after it ends and before the run records
// anything more, loses nothing it reported durable, and the
// next run resumes from exactly the checkpoint last reported. Every sequence then ends the pass with
// every message indexed once, masked and classified, its senders' statistics counted once, and no
// more than one page of rework for each crash (ADR-0017, ADR-0045).
func TestAKilledRunResumesFromItsCheckpoint(t *testing.T) {
	crash.Check(t, target(), config)
}

// The generator report for both draws (ADR-0069). Each kind of sequence the checks rest on is reached,
// stated as whether it is reached at all, not as an expected share.
func TestTheCrashSequencesReachEveryKind(t *testing.T) {
	minimums := map[string]float64{
		"a crash inside a page":                          0.001,
		"a crash between steps after a page was durable": 0.001,
		"a crash after a page's commit":                  0.001,
		"two crashes with no page between them":          0.001,
		"a run failed on a throttled page":               0.001,
	}
	for name, draw := range crash.Draws(target(), config) {
		t.Run(name, func(t *testing.T) {
			property.Report(t, draw, kind, minimums)
		})
	}
}

// kind classifies a case by the rarest telling thing its sequence does, in the order listed. A run
// fails on a throttled page only when a step asks for a page while the provider still has at least
// core.MaxAttempts throttles to give, so the classifier follows the order of the operations, how many
// throttles each step uses up and how many pages the mailbox has left, as the world does.
func kind(c crash.Case[setup]) string {
	left := max(1, (len(c.Setup.Messages)+c.Setup.PageSize-1)/max(c.Setup.PageSize, 1))
	pages, crashes, pending := 0, 0, 0
	lastWasCrash, back2back, inside, afterPage, afterCommitted, failedOnThrottle := false, false, false, false, false, false
	// step is one attempt to make the next page durable, which commits unless commit is false.
	step := func(commit bool) {
		if left == 0 {
			return
		}
		if pending >= core.MaxAttempts {
			failedOnThrottle = true
			pending -= core.MaxAttempts
			return
		}
		pending = 0
		if commit {
			left--
		}
	}
	for _, op := range c.Ops {
		switch op.Name {
		case crash.Crash:
			crashes++
			if lastWasCrash {
				back2back = true
			}
			switch {
			case op.Arg == insidePage:
				inside = true
				step(false)
			case op.Arg == afterCommit:
				afterCommitted = true
				step(true)
			case pages > 0:
				afterPage = true
			}
			lastWasCrash = true
			continue
		case "page":
			pages++
			step(true)
		case "throttle":
			pending = op.Arg
		}
		lastWasCrash = false
	}
	switch {
	case failedOnThrottle:
		return "a run failed on a throttled page"
	case back2back:
		return "two crashes with no page between them"
	case afterCommitted:
		return "a crash after a page's commit"
	case afterPage:
		return "a crash between steps after a page was durable"
	case inside:
		return "a crash inside a page"
	case crashes > 0:
		return "another crash"
	default:
		return "no crash"
	}
}
