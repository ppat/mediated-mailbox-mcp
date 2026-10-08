// Package attention decides Home's "worth a look" cards from the recorded state the shell reads and the
// thresholds the configuration sets (docs/UI.md section 8.1). The rules are derived at read time and
// carry no state and no verb (ADR-0056). Every card leaves here worded, so the browser renders text it
// does not compose. Times arrive as RFC 3339 instants in UTC, as the read API writes them, because a
// pure core imports no time package.
package attention

import (
	"slices"
	"strconv"
	"strings"
)

// The rules' identifiers, as the attention endpoint sends them.
const (
	Backlog    = "backlog"
	Masking    = "masking"
	BodyServes = "body_serves"
	SyncGap    = "sync_gap"
)

// Rules are the rules this package decides, in the order of section 8.1's table, which also orders
// cards with the same since.
func Rules() []string { return []string{Backlog, Masking, BodyServes, SyncGap} }

// BaselineDays is how many whole UTC days the body-serve rule takes its median over.
const BaselineDays = 7

// Thresholds are the rules' configured thresholds (docs/UI.md section 18.1). A threshold of 0 disables
// its rule.
type Thresholds struct {
	// BacklogShare is a percent of the corpus.
	BacklogShare float64
	// MaskCount is a count of events in the masking rule's 7 days.
	MaskCount int64
	// ServeFactor is a multiple of the median of daily serves.
	ServeFactor float64
	// GapDays is how many days back the sync-gap rule reads.
	GapDays int64
}

// Inputs are the recorded state the shell reads for the rules, as of one read transaction.
type Inputs struct {
	// Pending and Messages are the messages pending scan and every message the index holds.
	Pending  int64
	Messages int64
	// Pairs are the sender and rule pairs whose masking events in the last 7 days exceed MaskCount. The
	// statement that reads them applies the threshold, so the pairs it can return stay few.
	Pairs []Pair
	// Serves are the body serves of the last 24 hours, with the daily counts of the baseline days that
	// had a serve. A baseline day missing from DayCounts had none.
	Serves Serves
	// Recoveries are the gap recoveries that succeeded within GapDays, oldest first.
	Recoveries []Recovery
}

// Pair is one sender and rule with its masking events and the first of them.
type Pair struct {
	Sender  string
	Rule    string
	Events  int64
	FirstAt string
}

// Serves are the bodies served in the last 24 hours, the first of them, and the counts of the baseline
// days that had a serve.
type Serves struct {
	Count     int64
	FirstAt   string
	DayCounts []int64
}

// Recovery is one gap recovery that succeeded. WindowSeconds and Reconciled are nil when its counters do
// not record them.
type Recovery struct {
	StartedAt     string
	FinishedAt    string
	WindowSeconds *int64
	Reconciled    *int64
}

// Card is one worth-a-look card. Since is empty for a card that is a state as of the read. Sender and
// MaskRule name a masking card's pair, from which the shell builds its link.
type Card struct {
	Rule     string
	What     string
	Number   int64
	Since    string
	Sentence string
	Sender   string
	MaskRule string
}

// Cards decides every card the inputs fire under the thresholds, in section 8.1's order, which puts
// a card with no since first, then orders by since, newest first, then in the rules' order, masking
// cards by sender and rule.
func Cards(t Thresholds, in Inputs) []Card {
	var cards []Card
	if c, ok := backlog(t, in); ok {
		cards = append(cards, c)
	}
	if t.MaskCount > 0 {
		for _, p := range in.Pairs {
			if p.Events > t.MaskCount {
				cards = append(cards, masking(p))
			}
		}
	}
	if c, ok := serves(t, in.Serves); ok {
		cards = append(cards, c)
	}
	if c, ok := gap(t, in.Recoveries); ok {
		cards = append(cards, c)
	}
	slices.SortStableFunc(cards, order)
	return cards
}

func order(a, b Card) int {
	switch {
	case a.Since == "" && b.Since != "":
		return -1
	case a.Since != "" && b.Since == "":
		return 1
	case a.Since != b.Since:
		// RFC 3339 instants in UTC written to the second order as strings do.
		return strings.Compare(b.Since, a.Since)
	}
	if r := slices.Index(Rules(), a.Rule) - slices.Index(Rules(), b.Rule); r != 0 {
		return r
	}
	if s := strings.Compare(a.Sender, b.Sender); s != 0 {
		return s
	}
	return strings.Compare(a.MaskRule, b.MaskRule)
}

// backlog fires when the messages pending scan are above BacklogShare percent of the corpus. It has no
// since, because no column records when a message became pending.
func backlog(t Thresholds, in Inputs) (Card, bool) {
	if t.BacklogShare <= 0 || in.Messages <= 0 || float64(in.Pending)*100 <= t.BacklogShare*float64(in.Messages) {
		return Card{}, false
	}
	return Card{
		Rule:   Backlog,
		What:   "Scan backlog",
		Number: in.Pending,
		Sentence: Count(in.Pending) + " messages are pending scan (" + Share(in.Pending, in.Messages) +
			"% of the corpus). Every pending message denies its body until scanned, which reads to the agent like a permission problem.",
	}, true
}

func masking(p Pair) Card {
	return Card{
		Rule:     Masking,
		What:     "Masking",
		Number:   p.Events,
		Since:    p.FirstAt,
		Sender:   p.Sender,
		MaskRule: p.Rule,
		Sentence: "Masking fired " + Count(p.Events) + " times on " + p.Sender + " this week, all under " + p.Rule +
			". A sender masked this often under one rule is worth checking for an over-mask.",
	}
}

// Median is the median of the baseline days' serves, a day missing from counts having had none. Seven
// days have one middle value, so the median is always a whole count. A count beyond the seventh is
// never dropped, so a read over a wrong window moves the median rather than hiding a day, and the
// median of an even number of days is the upper of its two middle values.
func Median(counts []int64) int64 {
	days := make([]int64, max(BaselineDays, len(counts)))
	copy(days, counts)
	slices.Sort(days)
	return days[len(days)/2]
}

// serves fires when the bodies served in the last 24 hours are more than ServeFactor times the median.
// A median of 0 fires on any serve, which section 8.1 keeps on purpose.
func serves(t Thresholds, s Serves) (Card, bool) {
	median := Median(s.DayCounts)
	if t.ServeFactor <= 0 || s.Count == 0 || float64(s.Count) <= t.ServeFactor*float64(median) {
		return Card{}, false
	}
	return Card{
		Rule:   BodyServes,
		What:   "Body serves",
		Number: s.Count,
		Since:  s.FirstAt,
		Sentence: Count(s.Count) + " bodies were served in 24 hours against a 7-day median of " + Count(median) +
			". Body-serve volume beyond triage plausibility is the anomaly the design watches for.",
	}, true
}

// gap fires on any gap recovery that succeeded within GapDays, the recoveries the shell reads. Its
// sentence words the latest, and leaves out the window and reconciled clause its counters lack.
func gap(t Thresholds, rs []Recovery) (Card, bool) {
	if t.GapDays <= 0 || len(rs) == 0 {
		return Card{}, false
	}
	latest := rs[len(rs)-1]
	at := latest.FinishedAt
	if at == "" {
		at = latest.StartedAt
	}
	sentence := "Delta sync recovered from a cursor gap on " + Minute(at)
	if latest.WindowSeconds != nil && latest.Reconciled != nil {
		sentence += ", re-enumerating a " + Duration(*latest.WindowSeconds) + " window and reconciling " +
			Count(*latest.Reconciled) + " messages"
	}
	return Card{
		Rule:     SyncGap,
		What:     "Sync gap",
		Number:   int64(len(rs)),
		Since:    rs[0].StartedAt,
		Sentence: sentence + ". A repeated gap means the cadence or the cursor lifetime needs attention.",
	}, true
}

// Count writes a count with a thousands separator, 12,480 (docs/UI.md section 11).
func Count(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return sign + b.String()
}

// Share writes part of whole as a percent with one decimal and no sign, 14.8. A share of nothing is
// 0.0.
func Share(part, whole int64) string {
	if whole == 0 {
		return "0.0"
	}
	return strconv.FormatFloat(float64(part)*100/float64(whole), 'f', 1, 64)
}

// Duration writes a span of seconds in its largest two units, leaving out a second unit that is zero, so
// 1h 10m, 6h and 12s. A span under a second is 0s.
func Duration(seconds int64) string {
	units := []struct {
		name string
		size int64
	}{{"d", 86_400}, {"h", 3_600}, {"m", 60}, {"s", 1}}
	seconds = max(0, seconds)
	for i, u := range units {
		if seconds < u.size {
			continue
		}
		out := strconv.FormatInt(seconds/u.size, 10) + u.name
		if i+1 < len(units) {
			next := units[i+1]
			if rest := seconds % u.size / next.size; rest > 0 {
				out += " " + strconv.FormatInt(rest, 10) + next.name
			}
		}
		return out
	}
	return "0s"
}

// Minute writes an RFC 3339 instant in UTC as its date and time to the minute with the Z suffix,
// 2026-09-10 10:12Z.
func Minute(instant string) string {
	if len(instant) < 16 {
		return instant
	}
	return instant[:10] + " " + instant[11:16] + "Z"
}
