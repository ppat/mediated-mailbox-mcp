package release_test

import (
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/release"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// fragments are what a generated text is built from, chosen to reach every edge of the match. They
// are the delimiters' words whole, split at several places, in either case and overlapping their
// own ends, each letter of the words alone in either case, so a match can be spelled one letter at a
// time, the long s, which folds to s and so can stand for the words' s as a letter two bytes wide,
// the Kelvin sign and the dotless and dotted i, letters wider than a byte that fold to no letter of
// the words, separators and a zero-width space between letters, other multi-byte letters, and the
// replacement's own text.
var fragments = []string{
	"untrusted", "content", "UNTRUSTED", "Content", "untrustedcontent", "Untrusted Content",
	"un", "trus", "ted", "con", "tent", "ntrustedcontentu", "untrustedcontentuntrusted",
	"u", "n", "t", "r", "s", "e", "d", "c", "o", "U", "N", "T", "R", "S", "E", "D", "C", "O",
	"\u212a", "\u017f", "\u0131", "\u0130", " ", "-", "*", "\n", "\u200b", "x", "\u00e9", "\u65e5",
	"[delimiter text removed]",
}

// forms are the letters each letter of the words is spelled with in a drawn spelling, its two cases
// and, for s, the long s.
var forms = map[byte][]string{'s': {"s", "S", "\u017f"}}

// between are what a drawn spelling puts between two of its letters, nothing most often.
var between = []string{"", "", "", "", " ", "-", "\u200b", "\n"}

// drawSpelling draws the words spelled one letter at a time, each letter in either case or its
// look-alike, with a separator or nothing between letters. Joined fragments alone reach a match
// holding the long s only by drawing its sixteen letters one by one in order, which a run never does,
// so a fragment is a drawn spelling about one time in len(fragments)+1.
func drawSpelling(t *rapid.T, label string) string {
	const keyword = "untrustedcontent"
	var b strings.Builder
	for k := range len(keyword) {
		if k > 0 {
			b.WriteString(rapid.SampledFrom(between).Draw(t, label+" between"))
		}
		c := keyword[k]
		choices, ok := forms[c]
		if !ok {
			choices = []string{string(c), strings.ToUpper(string(c))}
		}
		b.WriteString(rapid.SampledFrom(choices).Draw(t, label+" letter"))
	}
	return b.String()
}

// differential is one generated text, the fragments it joins.
type differential struct {
	Fragments []string
}

func drawDifferential(t *rapid.T) differential {
	fragment := rapid.Custom(func(t *rapid.T) string {
		if i := rapid.IntRange(0, len(fragments)).Draw(t, "fragment"); i < len(fragments) {
			return fragments[i]
		}
		return drawSpelling(t, "spelling")
	})
	return differential{Fragments: rapid.SliceOfN(fragment, 0, 40).Draw(t, "fragments")}
}

func (d differential) text() string { return strings.Join(d.Fragments, "") }

// A differential test, kind 2 of ADR-0055. The release step's neutralizer, which keeps a window of
// the last letters, replaces exactly what the neutralizer it replaced did, kept below as the second
// implementation, in every released snippet. The snippet is released neutralized and unwrapped, so
// it shows the neutralizer's output alone.
func TestTheWindowedNeutralizerReplacesWhatTheFullScanDid(t *testing.T) {
	s := scanner(t)
	property.Check(t, drawDifferential, func(t rapid.TB, d differential) {
		text := d.text()
		got, ok := release.Decide(release.Content{Body: "a body", Snippet: text}, sensitivity.Scanned(), s).Released()
		if !ok {
			t.Fatalf("%q: the content was withheld", text)
		}
		if want := fullScan(text); got.Snippet != want {
			t.Fatalf("%q: released %q, the full scan gives %q", text, got.Snippet, want)
		}
	})
}

// The generator report for the property above. A text is replaced when the full scan changes it,
// and a replaced stretch holds a wide letter when one of the letters it spells the words with is
// wider than a byte, where the window's byte ranges differ from the letters' indices.
func TestTheWindowedNeutralizerReplacesWhatTheFullScanDidMix(t *testing.T) {
	property.Report(t, drawDifferential, func(d differential) string {
		text := d.text()
		switch _, matches := fullScanMatches(text); {
		case len(matches) == 0:
			return "unchanged"
		case slices.ContainsFunc(matches, holdsWideLetter):
			return "replaced, a wide letter inside a match"
		case len(matches) > 1:
			return "replaced more than once"
		default:
			return "replaced once"
		}
	}, map[string]float64{
		"unchanged": 0.05, "replaced once": 0.05, "replaced more than once": 0.01,
		"replaced, a wide letter inside a match": 0.02,
	})
}

// holdsWideLetter reports whether a stretch holds a letter wider than a byte.
func holdsWideLetter(stretch string) bool {
	for _, r := range stretch {
		if unicode.IsLetter(r) && utf8.RuneLen(r) > 1 {
			return true
		}
	}
	return false
}

// fullScan is the neutralizer the windowed one replaced, as it stood, which listed the byte range of
// every letter of the text before sliding a window of len(keyword) letters over the list.
func fullScan(text string) string {
	replaced, _ := fullScanMatches(text)
	return replaced
}

// fullScanMatches is fullScan, also returning each stretch it replaced.
func fullScanMatches(text string) (string, []string) {
	const keyword, replacement = "untrustedcontent", "[delimiter text removed]"
	type letter struct{ start, end int }
	var letters []letter
	for i, r := range text {
		if unicode.IsLetter(r) {
			letters = append(letters, letter{i, i + utf8.RuneLen(r)})
		}
	}
	spells := func(ls []letter) bool {
		for k, l := range ls {
			r, _ := utf8.DecodeRuneInString(text[l.start:l.end])
			if !foldsTo(r, rune(keyword[k])) {
				return false
			}
		}
		return true
	}
	var b strings.Builder
	var matches []string
	written := 0
	for i := 0; i+len(keyword) <= len(letters); {
		if !spells(letters[i : i+len(keyword)]) {
			i++
			continue
		}
		b.WriteString(text[written:letters[i].start])
		b.WriteString(replacement)
		written = letters[i+len(keyword)-1].end
		matches = append(matches, text[letters[i].start:written])
		i += len(keyword)
	}
	b.WriteString(text[written:])
	return b.String(), matches
}

func foldsTo(r, c rune) bool {
	for f := r; ; {
		if f == c {
			return true
		}
		if f = unicode.SimpleFold(f); f == r {
			return false
		}
	}
}
