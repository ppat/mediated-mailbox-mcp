package scan

import (
	"unicode"
	"unicode/utf8"
)

// automaton finds every occurrence of a set of words in a text in one pass, the Aho-Corasick
// automaton ADR-0005 names for the keyword layer. Matching folds ASCII letters to lower case, and
// other bytes match as written.
type automaton struct {
	next  []map[byte]int
	fail  []int
	words [][]int
	sizes []int
}

// match is one occurrence of a word, as byte offsets into the text searched.
type match struct {
	start, end int
	word       int
}

func fold(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

// newAutomaton builds the automaton over words, each identified by its index.
func newAutomaton(words []string) automaton {
	a := automaton{next: []map[byte]int{{}}, fail: []int{0}, words: [][]int{nil}}
	for i, w := range words {
		node := 0
		for j := range len(w) {
			b := fold(w[j])
			child, ok := a.next[node][b]
			if !ok {
				child = len(a.next)
				a.next = append(a.next, map[byte]int{})
				a.fail = append(a.fail, 0)
				a.words = append(a.words, nil)
				a.next[node][b] = child
			}
			node = child
		}
		a.words[node] = append(a.words[node], i)
		a.sizes = append(a.sizes, len(w))
	}
	queue := make([]int, 0, len(a.next))
	for _, child := range a.next[0] {
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for b, child := range a.next[node] {
			f := a.fail[node]
			for f != 0 && a.next[f][b] == 0 {
				f = a.fail[f]
			}
			if target, ok := a.next[f][b]; ok && target != child {
				a.fail[child] = target
			}
			a.words[child] = append(a.words[child], a.words[a.fail[child]]...)
			queue = append(queue, child)
		}
	}
	return a
}

// find returns every occurrence of a word in text. With bounded, an occurrence must start and end on
// a word boundary, so a word inside a longer word does not match.
func (a automaton) find(text string, bounded bool) []match {
	var found []match
	node := 0
	for i := range len(text) {
		b := fold(text[i])
		for node != 0 && a.next[node][b] == 0 {
			node = a.fail[node]
		}
		node = a.next[node][b]
		for _, w := range a.words[node] {
			start := i + 1 - a.sizes[w]
			if !bounded || boundaryBefore(text, start) && boundaryAfter(text, i+1) {
				found = append(found, match{start: start, end: i + 1, word: w})
			}
		}
	}
	return found
}

// wordRune reports whether r belongs to a word, a letter or a digit in any script.
func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// boundaryBefore reports whether offset i in text starts a word, with no letter or digit just
// before it.
func boundaryBefore(text string, i int) bool {
	if i <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return !wordRune(r)
}

// boundaryAfter reports whether offset i in text ends a word, with no letter or digit at it.
func boundaryAfter(text string, i int) bool {
	if i >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return !wordRune(r)
}
