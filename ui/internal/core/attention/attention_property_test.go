package attention_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
)

// pairArgs are one sender and rule's masking events, the first of them on a day of September 2026.
type pairArgs struct {
	Sender, Rule string
	Events       int64
	Day          int
}

// recoveryArgs are one gap recovery, started on a day of September 2026, its counters recorded or not.
type recoveryArgs struct {
	Day                int
	Recorded           bool
	Window, Reconciled int64
}

// disableArgs are the thresholds, the recorded state the rules read, and the rule whose threshold is
// set to 0. Each threshold is drawn as 0 or as a positive value on its own, so other rules may be
// disabled too.
type disableArgs struct {
	BacklogShare  float64
	MaskCount     int64
	ServeFactor   float64
	GapDays       int64
	Disabled      string
	Pending       int64
	Messages      int64
	Pairs         []pairArgs
	Serves        int64
	ServesDay     int
	DayCounts     []int64
	RecoveredDays []recoveryArgs
}

func stamp(day int) string {
	d := strconv.Itoa(day)
	if day < 10 {
		d = "0" + d
	}
	return "2026-09-" + d + "T10:00:00Z"
}

func drawDisable(t *rapid.T) disableArgs {
	threshold := func(label string, positive *rapid.Generator[int64]) int64 {
		if rapid.IntRange(0, 4).Draw(t, label+" disabled") == 0 {
			return 0
		}
		return positive.Draw(t, label)
	}
	messages := rapid.Int64Range(0, 2_000).Draw(t, "messages")
	return disableArgs{
		BacklogShare: float64(threshold("backlog share", rapid.Int64Range(1, 50))),
		MaskCount:    threshold("mask count", rapid.Int64Range(1, 30)),
		ServeFactor:  float64(threshold("serve factor", rapid.Int64Range(1, 4))),
		GapDays:      threshold("gap days", rapid.Int64Range(1, 30)),
		Disabled:     rapid.SampledFrom(attention.Rules()).Draw(t, "disabled"),
		Pending:      rapid.Int64Range(0, messages).Draw(t, "pending"),
		Messages:     messages,
		Pairs: rapid.SliceOfN(rapid.Custom(func(t *rapid.T) pairArgs {
			return pairArgs{
				Sender: rapid.SampledFrom([]string{"bank.example", "shop.example", "<b>x</b>.example"}).Draw(t, "sender"),
				Rule:   rapid.SampledFrom([]string{"content.mfa", "content.link"}).Draw(t, "rule"),
				Events: rapid.Int64Range(1, 60).Draw(t, "events"),
				Day:    rapid.IntRange(1, 9).Draw(t, "day"),
			}
		}), 0, 4).Draw(t, "pairs"),
		Serves:    rapid.Int64Range(0, 50).Draw(t, "serves"),
		ServesDay: rapid.IntRange(9, 10).Draw(t, "serves day"),
		DayCounts: rapid.SliceOfN(rapid.Int64Range(1, 20), 0, attention.BaselineDays).Draw(t, "day counts"),
		RecoveredDays: rapid.SliceOfN(rapid.Custom(func(t *rapid.T) recoveryArgs {
			return recoveryArgs{
				Day:        rapid.IntRange(1, 10).Draw(t, "day"),
				Recorded:   rapid.Bool().Draw(t, "recorded"),
				Window:     rapid.Int64Range(60, 86_400).Draw(t, "window"),
				Reconciled: rapid.Int64Range(0, 5_000).Draw(t, "reconciled"),
			}
		}), 0, 3).Draw(t, "recoveries"),
	}
}

func thresholds(a disableArgs) attention.Thresholds {
	return attention.Thresholds{BacklogShare: a.BacklogShare, MaskCount: a.MaskCount, ServeFactor: a.ServeFactor, GapDays: a.GapDays}
}

// disabled is the thresholds with one rule's set to 0, the configuration value that disables it.
func disabled(th attention.Thresholds, rule string) attention.Thresholds {
	switch rule {
	case attention.Backlog:
		th.BacklogShare = 0
	case attention.Masking:
		th.MaskCount = 0
	case attention.BodyServes:
		th.ServeFactor = 0
	case attention.SyncGap:
		th.GapDays = 0
	}
	return th
}

func inputs(a disableArgs) attention.Inputs {
	in := attention.Inputs{
		Pending: a.Pending, Messages: a.Messages,
		Serves: attention.Serves{Count: a.Serves, DayCounts: a.DayCounts},
	}
	if a.Serves > 0 {
		in.Serves.FirstAt = stamp(a.ServesDay)
	}
	for _, p := range a.Pairs {
		in.Pairs = append(in.Pairs, attention.Pair{Sender: p.Sender, Rule: p.Rule, Events: p.Events, FirstAt: stamp(p.Day)})
	}
	recoveries := slices.Clone(a.RecoveredDays)
	slices.SortStableFunc(recoveries, func(x, y recoveryArgs) int { return x.Day - y.Day })
	for _, r := range recoveries {
		rec := attention.Recovery{StartedAt: stamp(r.Day), FinishedAt: stamp(r.Day)}
		if r.Recorded {
			rec.WindowSeconds, rec.Reconciled = &r.Window, &r.Reconciled
		}
		in.Recoveries = append(in.Recoveries, rec)
	}
	return in
}

func without(cards []attention.Card, rule string) []attention.Card {
	return slices.DeleteFunc(slices.Clone(cards), func(c attention.Card) bool { return c.Rule == rule })
}

// A threshold of 0 disables its rule (docs/UI.md sections 8.1 and 18.1), stated as a metamorphic
// relation that leaves how each rule fires to the example tests (ADR-0055). Setting one rule's
// threshold to 0 yields exactly the cards the thresholds as drawn yield, less that rule's, so the
// disabled rule yields no card and every other rule's cards, and their order, are untouched.
func TestAThresholdOfZeroDisablesOnlyItsRule(t *testing.T) {
	property.Check(t, drawDisable, func(t rapid.TB, a disableArgs) {
		in := inputs(a)
		got := attention.Cards(disabled(thresholds(a), a.Disabled), in)
		for _, c := range got {
			if c.Rule == a.Disabled {
				t.Fatalf("%+v: the rule %s, whose threshold is 0, yielded %+v", a, a.Disabled, c)
			}
		}
		want := without(attention.Cards(thresholds(a), in), a.Disabled)
		// No card at all is the same answer whether it arrives as an empty list or none.
		if diff := cmp.Diff(want, got, compare.Options, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("%+v: disabling %s changed the other rules' cards (-want +got):\n%s", a, a.Disabled, diff)
		}
	})
}

// The generated cases must often give the disabled rule a card it loses, with and without other rules'
// cards beside it, or the relation would hold over runs where disabling removes nothing.
func TestAThresholdOfZeroDisablesOnlyItsRuleMix(t *testing.T) {
	property.Report(t, drawDisable, func(a disableArgs) string {
		cards := attention.Cards(thresholds(a), inputs(a))
		lost := slices.ContainsFunc(cards, func(c attention.Card) bool { return c.Rule == a.Disabled })
		others := len(without(cards, a.Disabled)) > 0
		switch {
		case lost && others:
			return "the disabled rule's card beside others"
		case lost:
			return "the disabled rule's card alone"
		case others:
			return "other rules' cards only"
		default:
			return "no card"
		}
	}, map[string]float64{
		"the disabled rule's card beside others": 0.1,
		"the disabled rule's card alone":         0.02,
		"other rules' cards only":                0.05,
	})
}
