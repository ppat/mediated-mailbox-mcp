package attention_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
)

// starting are section 8.1's starting thresholds, written out here rather than read from the
// configuration's defaults, so a changed default does not move what the tests expect.
func starting() attention.Thresholds {
	return attention.Thresholds{BacklogShare: 5, MaskCount: 20, ServeFactor: 2, GapDays: 7}
}

func ptr(n int64) *int64 { return &n }

func rules(cards []attention.Card) []string {
	out := []string{}
	for _, c := range cards {
		out = append(out, c.Rule)
	}
	return out
}

func TestTheBacklogFiresAboveItsShareOfTheCorpus(t *testing.T) {
	cases := []struct {
		name              string
		pending, messages int64
		share             float64
		want              []attention.Card
	}{
		{name: "above", pending: 6, messages: 100, share: 5, want: []attention.Card{{
			Rule: "backlog", What: "Scan backlog", Number: 6,
			Sentence: "6 messages are pending scan (6.0% of the corpus). Every pending message denies its body until scanned, which reads to the agent like a permission problem.",
		}}},
		{name: "at the threshold", pending: 5, messages: 100, share: 5},
		{name: "below", pending: 4, messages: 100, share: 5},
		{name: "an empty index", pending: 0, messages: 0, share: 5},
		{name: "disabled", pending: 100, messages: 100, share: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			th := starting()
			th.BacklogShare = c.share
			got := attention.Cards(th, attention.Inputs{Pending: c.pending, Messages: c.messages})
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("cards (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMaskingFiresPerPairAboveItsCount(t *testing.T) {
	pairs := []attention.Pair{
		{Sender: "bank.example", Rule: "content.mfa", Events: 21, FirstAt: "2026-09-05T10:00:00Z"},
		{Sender: "bank.example", Rule: "content.link", Events: 20, FirstAt: "2026-09-06T10:00:00Z"},
		{Sender: "shop.example", Rule: "content.mfa", Events: 1200, FirstAt: "2026-09-05T10:00:00Z"},
	}
	got := attention.Cards(starting(), attention.Inputs{Pairs: pairs})
	want := []attention.Card{
		{
			Rule: "masking", What: "Masking", Number: 21, Since: "2026-09-05T10:00:00Z", Sender: "bank.example", MaskRule: "content.mfa",
			Sentence: "Masking fired 21 times on bank.example this week, all under content.mfa. A sender masked this often under one rule is worth checking for an over-mask.",
		},
		{
			Rule: "masking", What: "Masking", Number: 1200, Since: "2026-09-05T10:00:00Z", Sender: "shop.example", MaskRule: "content.mfa",
			Sentence: "Masking fired 1,200 times on shop.example this week, all under content.mfa. A sender masked this often under one rule is worth checking for an over-mask.",
		},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("cards (-want +got):\n%s", diff)
	}
	th := starting()
	th.MaskCount = 0
	if got := attention.Cards(th, attention.Inputs{Pairs: pairs}); len(got) != 0 {
		t.Errorf("a masking count of 0 fired %v", rules(got))
	}
}

func TestTheMedianCountsADayWithoutAServeAsZero(t *testing.T) {
	cases := []struct {
		counts []int64
		want   int64
	}{
		{nil, 0},
		{[]int64{9, 9, 9}, 0},
		{[]int64{1, 2, 3, 4}, 1},
		{[]int64{5, 1, 4, 2, 3, 7, 6}, 4},
		// An eighth day's count is kept, never dropped.
		{[]int64{1, 1, 1, 1, 1, 1, 1, 500}, 1},
		{[]int64{9, 9, 9, 9, 1, 1, 1, 1}, 9},
	}
	for _, c := range cases {
		if got := attention.Median(c.counts); got != c.want {
			t.Errorf("the median of %v over seven days is %d, want %d", c.counts, got, c.want)
		}
	}
}

func TestBodyServesFireAboveTheFactorOfTheMedian(t *testing.T) {
	cases := []struct {
		name   string
		serves attention.Serves
		factor float64
		want   []attention.Card
	}{
		{
			name:   "above",
			serves: attention.Serves{Count: 9, FirstAt: "2026-09-09T11:00:00Z", DayCounts: []int64{4, 4, 4, 4, 4, 4, 4}},
			factor: 2,
			want: []attention.Card{{
				Rule: "body_serves", What: "Body serves", Number: 9, Since: "2026-09-09T11:00:00Z",
				Sentence: "9 bodies were served in 24 hours against a 7-day median of 4. Body-serve volume beyond triage plausibility is the anomaly the design watches for.",
			}},
		},
		{name: "at twice the median", serves: attention.Serves{Count: 8, FirstAt: "x", DayCounts: []int64{4, 4, 4, 4, 4, 4, 4}}, factor: 2},
		{
			name:   "four quiet days hold the median at 0",
			serves: attention.Serves{Count: 1, FirstAt: "2026-09-09T11:00:00Z", DayCounts: []int64{50, 50, 50}},
			factor: 2,
			want: []attention.Card{{
				Rule: "body_serves", What: "Body serves", Number: 1, Since: "2026-09-09T11:00:00Z",
				Sentence: "1 bodies were served in 24 hours against a 7-day median of 0. Body-serve volume beyond triage plausibility is the anomaly the design watches for.",
			}},
		},
		{name: "no serve", serves: attention.Serves{}, factor: 2},
		{name: "disabled", serves: attention.Serves{Count: 1000, FirstAt: "x"}, factor: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			th := starting()
			th.ServeFactor = c.factor
			got := attention.Cards(th, attention.Inputs{Serves: c.serves})
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("cards (-want +got):\n%s", diff)
			}
		})
	}
}

func TestASyncGapIsOneCardWordedFromTheLatestRecovery(t *testing.T) {
	recoveries := []attention.Recovery{
		{StartedAt: "2026-09-04T01:00:00Z", FinishedAt: "2026-09-04T01:05:00Z", WindowSeconds: ptr(3600), Reconciled: ptr(3)},
		{StartedAt: "2026-09-08T00:00:00Z", FinishedAt: "2026-09-08T00:10:00Z", WindowSeconds: ptr(6*3600 + 30*60), Reconciled: ptr(1041)},
	}
	got := attention.Cards(starting(), attention.Inputs{Recoveries: recoveries})
	want := []attention.Card{{
		Rule: "sync_gap", What: "Sync gap", Number: 2, Since: "2026-09-04T01:00:00Z",
		Sentence: "Delta sync recovered from a cursor gap on 2026-09-08 00:10Z, re-enumerating a 6h 30m window and reconciling 1,041 messages. A repeated gap means the cadence or the cursor lifetime needs attention.",
	}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("cards (-want +got):\n%s", diff)
	}

	// A recovery whose counters lack the window is worded without that clause.
	unrecorded := []attention.Recovery{{StartedAt: "2026-09-08T00:00:00Z", FinishedAt: "2026-09-08T00:10:00Z", Reconciled: ptr(4)}}
	got = attention.Cards(starting(), attention.Inputs{Recoveries: unrecorded})
	if len(got) != 1 || got[0].Sentence != "Delta sync recovered from a cursor gap on 2026-09-08 00:10Z. A repeated gap means the cadence or the cursor lifetime needs attention." {
		t.Errorf("a recovery without its window was worded %v", got)
	}

	th := starting()
	th.GapDays = 0
	if got := attention.Cards(th, attention.Inputs{Recoveries: recoveries}); len(got) != 0 {
		t.Errorf("a gap window of 0 days fired %v", rules(got))
	}
}

func TestCardsAreNewestFirstWithTheBacklogAhead(t *testing.T) {
	in := attention.Inputs{
		Pending: 50, Messages: 100,
		Pairs: []attention.Pair{
			{Sender: "b.example", Rule: "r", Events: 30, FirstAt: "2026-09-08T00:00:00Z"},
			{Sender: "a.example", Rule: "r", Events: 30, FirstAt: "2026-09-08T00:00:00Z"},
		},
		Serves:     attention.Serves{Count: 3, FirstAt: "2026-09-09T12:00:00Z"},
		Recoveries: []attention.Recovery{{StartedAt: "2026-09-08T00:00:00Z", FinishedAt: "2026-09-08T00:01:00Z"}},
	}
	got := attention.Cards(starting(), in)
	type key struct{ Rule, Sender string }
	keys := []key{}
	for _, c := range got {
		keys = append(keys, key{c.Rule, c.Sender})
	}
	want := []key{{"backlog", ""}, {"body_serves", ""}, {"masking", "a.example"}, {"masking", "b.example"}, {"sync_gap", ""}}
	if diff := cmp.Diff(want, keys, compare.Options); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
}

func TestTheFormatsFollowSection11(t *testing.T) {
	for in, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 12480: "12,480", 1234567: "1,234,567"} {
		if got := attention.Count(in); got != want {
			t.Errorf("Count(%d) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[int64]string{0: "0s", 12: "12s", 4200: "1h 10m", 21600: "6h", 1209600: "14d", 90061: "1d 1h"} {
		if got := attention.Duration(in); got != want {
			t.Errorf("Duration(%d) = %q, want %q", in, got, want)
		}
	}
	if got := attention.Share(148, 1000); got != "14.8" {
		t.Errorf("Share = %q", got)
	}
	if got := attention.Minute("2026-09-10T10:12:59Z"); got != "2026-09-10 10:12Z" {
		t.Errorf("Minute = %q", got)
	}
}
