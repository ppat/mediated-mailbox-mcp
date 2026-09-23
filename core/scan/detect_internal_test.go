package scan

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The automaton finds every occurrence of every word, words sharing a prefix, words inside other
// words, and words found only by following a failure link included, and with bounded only those on
// word boundaries. The expected occurrences are what a search for each word on its own finds.
func TestAutomaton(t *testing.T) {
	a := newAutomaton([]string{"he", "she", "his", "hers", "code", "security code", "coder"})
	cases := []struct {
		name, text string
		bounded    bool
		want       []match
	}{
		{"overlapping words", "ushers", false, []match{{1, 4, 1}, {2, 4, 0}, {2, 6, 3}}},
		{"a word reached through a failure link", "shis", false, []match{{1, 4, 2}}},
		{"a word and its extension", "coder", false, []match{{0, 4, 4}, {0, 5, 6}}},
		{"a two-word phrase and its last word", "Security Code", true, []match{{0, 13, 5}, {9, 13, 4}}},
		{"a word inside a longer word, bounded", "barcode coder", true, []match{{8, 13, 6}}},
		{"a word next to a letter in another script", "codeé code", true, []match{{7, 11, 4}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, a.find(c.text, c.bounded), compare.Options, cmp.AllowUnexported(match{})); diff != "" {
				t.Errorf("find(%q) (-want +got):\n%s", c.text, diff)
			}
		})
	}
}

// Tier 2's score is the mean of its five features under equal weights, and each feature moves it.
func TestScore(t *testing.T) {
	s, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, text, token string
		want              float64
	}{
		{"near a trigger, mixed case and digits, six characters", "code 7GX4Q2", "7GX4Q2", (1 + 2.0/3 + 1 + 1 + 0) / 5},
		{"alone on its line", "7GX4Q2", "7GX4Q2", (1 + 2.0/3 + 1 + 0 + 1) / 5},
		{"in bold", "see **7GX4Q2** now", "7GX4Q2", (1 + 2.0/3 + 1 + 0 + 1) / 5},
		{"digits only", "see 419283756", "419283756", (1 + 1.0/3 + 1 + 0 + 0) / 5},
		{"four characters", "see A1B2", "A1B2", (1 + 2.0/3 + 0.5 + 0 + 0) / 5},
		{"repeated characters", "see aaaa11", "aaaa11", (0.9182958340544896/math.Log2(6) + 2.0/3 + 1 + 0 + 0) / 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tokens := tokenize(c.text)
			var triggers []int
			for _, m := range s.triggers.find(c.text, true) {
				triggers = append(triggers, tokenAt(tokens, m.start))
			}
			i := tokenAt(tokens, len(c.text)-len(c.token))
			for tokens[i].end-tokens[i].start != len(c.token) {
				i--
			}
			if got := s.score(c.text, tokens[i], i, triggers, layout(c.text)); math.Abs(got-c.want) > 1e-9 {
				t.Errorf("score of %q in %q is %v, want %v", c.token, c.text, got, c.want)
			}
		})
	}
}
