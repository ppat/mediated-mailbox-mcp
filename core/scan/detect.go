package scan

import (
	"math"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Flag is the content flag a span raises.
type Flag uint8

const (
	// OneTimeCode is a detected one-time code.
	OneTimeCode Flag = iota + 1
	// LoginLink is a detected login link.
	LoginLink
)

// Span is one detection in a subject, as byte offsets into it, with the rule and tier that made it.
// A span exists to mask the subject it was found in and is never stored, because an offset is
// derived from content (ADR-0009).
type Span struct {
	Start, End int
	Rule       string
	Tier       int
	Flag       Flag
}

// SubjectSpans returns every detection in subject, with tier 2 scoring against the subject threshold,
// and false when the Scanner was not built by New, so a caller masks everything rather than nothing
// (ADR-0003).
func (s Scanner) SubjectSpans(subject string) ([]Span, bool) {
	if !s.built {
		return nil, false
	}
	spans, _ := s.detect(subject, true)
	return spans, true
}

// detect applies tier 1, then tier 2 to what tier 1 left, and returns the spans in order with the
// highest tier evaluated. With subject, tier 2 scores against the subject threshold.
func (s Scanner) detect(text string, subject bool) ([]Span, int) {
	tokens := tokenize(text)
	var triggers []int
	for _, m := range s.triggers.find(text, true) {
		triggers = append(triggers, tokenAt(tokens, m.start))
	}
	slices.Sort(triggers)
	var spans []Span
	add := func(start, end int, rule string, tier int, flag Flag) {
		spans = append(spans, Span{Start: start, End: end, Rule: rule, Tier: tier, Flag: flag})
	}
	for _, r := range digitRuns(text) {
		if near(triggers, tokenAt(tokens, r.start), s.cfg.Window) {
			add(r.start, r.end, RuleTriggerWindow, 1, OneTimeCode)
		}
	}
	ls := layout(text)
	for li, ln := range ls.lines {
		line := text[ln.start:ln.end]
		for _, r := range digitRuns(line) {
			start, end := ln.start+r.start, ln.start+r.end
			switch {
			case ls.trimmed[li] == line[r.start:r.end]:
				add(start, end, RuleOwnLine, 1, OneTimeCode)
			case ls.heading[li] || ls.cell(li, line, r):
				add(start, end, RuleHeadingOrCell, 1, OneTimeCode)
			}
		}
	}
	// covered marks every byte a tier 1 span or a URL holds, so tier 2 skips them in one pass.
	covered := make([]bool, len(text))
	mark := func(start, end int) {
		for i := start; i < end; i++ {
			covered[i] = true
		}
	}
	for _, sp := range spans {
		mark(sp.Start, sp.End)
	}
	for _, u := range s.url.FindAllStringIndex(text, -1) {
		end := u[0] + len(strings.TrimRight(text[u[0]:u[1]], ".,;:!?"))
		mark(u[0], end)
		for _, rule := range s.linkRules(text[u[0]:end]) {
			add(u[0], end, rule, 1, LoginLink)
		}
	}
	threshold := s.cfg.Threshold
	if subject {
		threshold = s.cfg.SubjectThreshold
	}
	tier := 1
	for i, t := range tokens {
		if !candidate(text[t.start:t.end]) || slices.Contains(covered[t.start:t.end], true) {
			continue
		}
		tier = 2
		if s.score(text, t, i, triggers, ls) >= threshold {
			add(t.start, t.end, RuleScore, 2, OneTimeCode)
		}
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	return spans, tier
}

// segment is a range of byte offsets into a text.
type segment struct{ start, end int }

// tokenize returns the maximal runs of letters and digits in text, in any script.
func tokenize(text string) []segment {
	var tokens []segment
	start := -1
	for i, r := range text {
		if wordRune(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			tokens = append(tokens, segment{start, i})
			start = -1
		}
	}
	if start >= 0 {
		tokens = append(tokens, segment{start, len(text)})
	}
	return tokens
}

// tokenAt returns the index of the token holding offset, or of the next one after it.
func tokenAt(tokens []segment, offset int) int {
	return sort.Search(len(tokens), func(i int) bool { return tokens[i].end > offset })
}

// near reports whether a trigger token other than token lies within window tokens of it. The
// triggers are sorted.
func near(triggers []int, token, window int) bool {
	i := sort.SearchInts(triggers, token-window)
	for ; i < len(triggers) && triggers[i] <= token+window; i++ {
		if triggers[i] != token {
			return true
		}
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// separatorAt returns the length of the group separator at text[i:], or 0. A separator is a space, a
// hyphen, or the no-break, narrow no-break and thin spaces mail uses to keep a code from wrapping.
func separatorAt(text string, i int) int {
	for _, sep := range [...]string{" ", "-", "\u00a0", "\u202f", "\u2009"} {
		if strings.HasPrefix(text[i:], sep) {
			return len(sep)
		}
	}
	return 0
}

// digitRuns returns every run of 4 to 8 digits standing alone in text. A run may be split into
// groups of at least 3 digits by a single separator, as in 419 283 or 419-283.
func digitRuns(text string) []segment {
	var runs []segment
	for i := 0; i < len(text); {
		if !isDigit(text[i]) || !boundaryBefore(text, i) {
			i++
			continue
		}
		end, digits, group := i, 0, 0
		for end < len(text) {
			if isDigit(text[end]) {
				digits++
				group++
				end++
				continue
			}
			if n := separatorAt(text, end); n > 0 && group >= 3 && end+n+3 <= len(text) &&
				isDigit(text[end+n]) && isDigit(text[end+n+1]) && isDigit(text[end+n+2]) {
				group = 0
				end += n
				continue
			}
			break
		}
		if digits >= 4 && digits <= 8 && boundaryAfter(text, end) {
			runs = append(runs, segment{i, end})
		}
		i = end + 1
	}
	return runs
}

// textLayout is each line of a text with what the rules read of it, computed once so every lookup
// stays linear in the text's length.
type textLayout struct {
	lines   []segment
	trimmed []string
	heading []bool
	pipes   [][]int
}

// layout splits text into lines, without their line breaks.
func layout(text string) textLayout {
	var ls textLayout
	start := 0
	for i := range len(text) + 1 {
		if i < len(text) && text[i] != '\n' {
			continue
		}
		end := i
		if end > start && text[end-1] == '\r' {
			end--
		}
		line := text[start:end]
		ls.lines = append(ls.lines, segment{start, end})
		ls.trimmed = append(ls.trimmed, strings.Trim(line, " \t*_`>"))
		ls.heading = append(ls.heading, heading(line))
		var pipes []int
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			for j := range len(line) {
				if line[j] == '|' {
					pipes = append(pipes, j)
				}
			}
		}
		ls.pipes = append(ls.pipes, pipes)
		start = i + 1
	}
	return ls
}

// lineOf returns the index of the line holding offset.
func (ls textLayout) lineOf(offset int) int {
	return sort.Search(len(ls.lines), func(i int) bool { return ls.lines[i].end >= offset })
}

// heading reports whether line is a first- or second-level Markdown heading.
func heading(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	return strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ")
}

// cell reports whether run r is the whole content of a cell of line, the li-th line, a Markdown
// table row.
func (ls textLayout) cell(li int, line string, r segment) bool {
	pipes := ls.pipes[li]
	right := sort.SearchInts(pipes, r.end)
	if right == 0 || right == len(pipes) || pipes[right-1] >= r.start {
		return false
	}
	return strings.Trim(line[pipes[right-1]+1:pipes[right]], " \t*_`") == line[r.start:r.end]
}

// linkRules returns the login-link rules url breaks. A link word counts anywhere in the path or the
// query, inside a longer word too, as in /confirmation/ or /resetPassword/, and not in the host.
func (s Scanner) linkRules(url string) []string {
	rest := url[strings.Index(url, "://")+3:]
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}
	hostEnd := strings.IndexAny(rest, "/?")
	if hostEnd < 0 {
		hostEnd = len(rest)
	}
	afterHost := rest[hostEnd:]
	path, query, _ := strings.Cut(afterHost, "?")
	var rules []string
	segments := strings.Split(path, "/")
	if slices.ContainsFunc(segments, s.dense) && len(s.linkWords.find(afterHost, false)) > 0 {
		rules = append(rules, RuleLinkPath)
	}
	for _, pair := range strings.Split(query, "&") {
		key, value, _ := strings.Cut(pair, "=")
		if s.linkParams[strings.ToLower(key)] && s.dense(value) {
			rules = append(rules, RuleLinkQuery)
			break
		}
	}
	return rules
}

// dense reports whether v is long and varied enough to be a secret rather than a word.
func (s Scanner) dense(v string) bool {
	return len(v) >= s.cfg.DenseLength && entropy(v) >= s.cfg.DenseEntropy
}

// entropy returns the Shannon entropy of v in bits per byte.
func entropy(v string) float64 {
	if v == "" {
		return 0
	}
	var counts [256]int
	for i := range len(v) {
		counts[v[i]]++
	}
	var h float64
	for _, c := range counts {
		if c > 0 {
			p := float64(c) / float64(len(v))
			h -= p * math.Log2(p)
		}
	}
	return h
}

// candidate reports whether a token is one tier 2 scores, 4 to 10 ASCII letters and digits with at
// least one digit.
func candidate(tok string) bool {
	if len(tok) < 4 || len(tok) > 10 || utf8.RuneCountInString(tok) != len(tok) {
		return false
	}
	digit := false
	for i := range len(tok) {
		b := tok[i]
		switch {
		case isDigit(b):
			digit = true
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z':
		default:
			return false
		}
	}
	return digit
}

// score returns tier 2's weighted score for token t, the index-th token, between 0 and 1.
func (s Scanner) score(text string, t segment, index int, triggers []int, ls textLayout) float64 {
	tok := text[t.start:t.end]
	var lower, upper, digit float64
	for i := range len(tok) {
		switch b := tok[i]; {
		case isDigit(b):
			digit = 1
		case b >= 'a' && b <= 'z':
			lower = 1
		default:
			upper = 1
		}
	}
	lengthScore := 0.5
	if len(tok) >= 6 && len(tok) <= 10 {
		lengthScore = 1
	}
	var proximity, position float64
	if near(triggers, index, s.cfg.Window) {
		proximity = 1
	}
	if ls.placed(text, t) {
		position = 1
	}
	w := s.cfg.Weights
	total := w.Entropy*math.Min(1, entropy(tok)/math.Log2(float64(len(tok)))) +
		w.Mix*(lower+upper+digit)/3 +
		w.Length*lengthScore +
		w.Proximity*proximity +
		w.Position*position
	return total / (w.Entropy + w.Mix + w.Length + w.Proximity + w.Position)
}

// placed reports whether token t stands out by position, alone on its line, in a heading, or in
// bold.
func (ls textLayout) placed(text string, t segment) bool {
	li := ls.lineOf(t.start)
	tok := text[t.start:t.end]
	bold := func(mark string) bool {
		return t.start >= len(mark) && t.end+len(mark) <= len(text) &&
			text[t.start-len(mark):t.start] == mark && text[t.end:t.end+len(mark)] == mark
	}
	return ls.trimmed[li] == tok || ls.heading[li] || bold("**") || bold("__")
}
