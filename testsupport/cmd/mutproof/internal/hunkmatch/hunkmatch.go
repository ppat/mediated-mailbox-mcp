// Package hunkmatch finds where each hunk of a mutation patch applies, the way git apply finds it,
// so the mutation runner can refuse a patch with a hunk git could place somewhere other than where its
// author meant (ADR-0046).
//
// git apply looks for a hunk's preimage, its context and removed lines, starting at the line its
// header names and moving outward one line at a time, after it then before it, and applies it at the
// first match. A hunk whose preimage matches at one place lands there whatever the offset, which is
// how a patch survives an unrelated edit that moves lines. A hunk whose preimage matches at more than
// one place lands at the match nearest its header line, which an unrelated edit can make the wrong
// one, so Check refuses it.
//
// The rules below are git's own, from apply.c (apply_one_fragment, find_pos and match_fragment),
// with git apply's defaults: no reduced context, no whitespace option, and no conversion of the file
// git reads through core.autocrlf or the text and eol attributes, which this repository sets for no
// file a patch targets. The package's property test compares the places it finds with git apply's.
//
//   - A hunk whose header names its old start as line 0 or 1 must match at the start of the file,
//     and one with no context after its last changed line must match at the end of the file. Such
//     a hunk has at most one place.
//   - Lines are compared byte for byte, line ending included, with one exception. A preimage line
//     marked as having no newline at the end of the file can only be the hunk's last, and git
//     compares it as a prefix: it matches a line that starts with it and continues with nothing but
//     whitespace, a newline or a carriage return included, anywhere in the file. Only a hunk that
//     must match at the end of the file holds that line to the file's own last line, exactly.
//   - The hunks of a file are placed in order, each against the file as the hunks before it left
//     it, and no hunk matches over a line an earlier hunk wrote. A patch that names a file twice
//     carries the file from the first section to the second, a rename or copy carries it to its
//     new path, and a new file starts empty.
package hunkmatch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// hunk is one hunk of a file's diff, with every line carrying its own line ending, or none for a
// line marked as having no newline at the end of the file.
type hunk struct {
	header             string
	oldStart, newStart int
	pre, post          []string
	// trailing counts the context lines after the last changed line.
	trailing int
}

// fileDiff is one diff --git section. oldPath is empty for a new file and newPath for a deleted
// one.
type fileDiff struct {
	oldPath, newPath string
	copied           bool
	hunks            []hunk
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// parseDiff reads the diff --git sections of a patch, after the free text before the first one.
// It refuses a binary patch and a hunk whose lines do not add up to its header's counts, since it
// could not say where either applies.
func parseDiff(src []byte) ([]fileDiff, error) {
	lines := strings.SplitAfter(string(src), "\n")
	if lines[len(lines)-1] == "" {
		// What follows the last newline, which is no line.
		lines = lines[:len(lines)-1]
	}
	var files []fileDiff
	var cur *fileDiff
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		text := strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(text, "diff --git "):
			files = append(files, fileDiff{})
			cur = &files[len(files)-1]
		case cur == nil:
			// The free text before the first diff header.
		case strings.HasPrefix(text, "GIT binary patch"), strings.HasPrefix(text, "Binary files "):
			return nil, errors.New("a binary patch cannot be checked for where it applies")
		case strings.HasPrefix(text, "--- "):
			path, err := diffPath(strings.TrimPrefix(text, "--- "), "a/")
			if err != nil {
				return nil, err
			}
			cur.oldPath = path
		case strings.HasPrefix(text, "+++ "):
			path, err := diffPath(strings.TrimPrefix(text, "+++ "), "b/")
			if err != nil {
				return nil, err
			}
			cur.newPath = path
		case strings.HasPrefix(text, "rename from "), strings.HasPrefix(text, "copy from "):
			_, rest, _ := strings.Cut(text, " from ")
			path, err := unquotePath(rest)
			if err != nil {
				return nil, err
			}
			cur.oldPath = path
			cur.copied = strings.HasPrefix(text, "copy ")
		case strings.HasPrefix(text, "rename to "), strings.HasPrefix(text, "copy to "):
			_, rest, _ := strings.Cut(text, " to ")
			path, err := unquotePath(rest)
			if err != nil {
				return nil, err
			}
			cur.newPath = path
		case strings.HasPrefix(text, "@@"):
			h, next, err := parseHunk(lines, i)
			if err != nil {
				return nil, err
			}
			cur.hunks = append(cur.hunks, h)
			i = next - 1
		}
	}
	return files, nil
}

// parseHunk reads the hunk whose header is lines[start] and returns it with the index of the line
// after it.
func parseHunk(lines []string, start int) (hunk, int, error) {
	header := strings.TrimSuffix(lines[start], "\n")
	m := hunkHeader.FindStringSubmatch(header)
	if m == nil {
		return hunk{}, 0, fmt.Errorf("unreadable hunk header %q", header)
	}
	// A count the header leaves out is 1.
	n := [4]int{1, 1, 1, 1}
	for i, field := range []string{m[1], m[2], m[3], m[4]} {
		if field == "" {
			continue
		}
		v, err := strconv.Atoi(field)
		if err != nil {
			return hunk{}, 0, fmt.Errorf("unreadable hunk header %q: %w", header, err)
		}
		n[i] = v
	}
	h := hunk{header: header, oldStart: n[0], newStart: n[2]}
	oldLeft, newLeft := n[1], n[3]
	// last says which side the latest line went to, so a following no-newline marker strips the
	// right line: 'c' for both, '-' for the preimage, '+' for the postimage.
	var last byte
	changed := false
	i := start + 1
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, `\`) {
			if last == 0 {
				return hunk{}, 0, fmt.Errorf("hunk %q: a no-newline marker follows no line", header)
			}
			if last != '+' {
				h.pre[len(h.pre)-1] = strings.TrimSuffix(h.pre[len(h.pre)-1], "\n")
			}
			if last != '-' {
				h.post[len(h.post)-1] = strings.TrimSuffix(h.post[len(h.post)-1], "\n")
			}
			continue
		}
		if oldLeft == 0 && newLeft == 0 {
			break
		}
		// git reads an empty line inside a hunk as an empty context line.
		kind, body := byte(' '), "\n"
		if line != "\n" {
			kind, body = line[0], line[1:]
		}
		switch kind {
		case ' ':
			h.pre, h.post = append(h.pre, body), append(h.post, body)
			oldLeft, newLeft = oldLeft-1, newLeft-1
			last = 'c'
			if changed {
				h.trailing++
			}
		case '-':
			h.pre = append(h.pre, body)
			oldLeft--
			last, changed, h.trailing = '-', true, 0
		case '+':
			h.post = append(h.post, body)
			newLeft--
			last, changed, h.trailing = '+', true, 0
		default:
			return hunk{}, 0, fmt.Errorf("hunk %q: line %q is not a context, removed or added line", header, strings.TrimSuffix(line, "\n"))
		}
		if oldLeft < 0 || newLeft < 0 {
			return hunk{}, 0, fmt.Errorf("hunk %q holds more lines than its header counts", header)
		}
	}
	if oldLeft != 0 || newLeft != 0 {
		return hunk{}, 0, fmt.Errorf("hunk %q holds fewer lines than its header counts", header)
	}
	return h, i, nil
}

// diffPath reads the path of a --- or +++ line, which is /dev/null for a side that does not exist.
func diffPath(field, prefix string) (string, error) {
	// git ends the path with a tab when it holds a space.
	field, _, _ = strings.Cut(field, "\t")
	if field == "/dev/null" {
		return "", nil
	}
	path, err := unquotePath(field)
	if err != nil {
		return "", err
	}
	trimmed, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return "", fmt.Errorf("the path %q does not start with %s", path, prefix)
	}
	return trimmed, nil
}

// unquotePath reads a path git wrote in C-style quotes when it holds an unusual character.
func unquotePath(path string) (string, error) {
	if !strings.HasPrefix(path, `"`) {
		return path, nil
	}
	unquoted, err := strconv.Unquote(path)
	if err != nil {
		return "", fmt.Errorf("cannot read the quoted path %s: %w", path, err)
	}
	return unquoted, nil
}

// imageLine is a line of a file as the hunks placed so far leave it. patched marks a line an
// earlier hunk wrote, which git apply never matches a later hunk over.
type imageLine struct {
	text    string
	patched bool
}

// Hunk is where one hunk of a patch matches.
type Hunk struct {
	// Path is the file the hunk is read against, by its path before the patch.
	Path string
	// Number counts the hunks of the file's diff section from 1, and Header is the hunk's header.
	Number int
	Header string
	// Matches are the lines, counted from 1, at which the hunk's preimage matches the file as the
	// hunks before it leave it, under git apply's rules. Chosen is the one git apply takes, the match
	// nearest the line the header names, after it when two are as near, and 0 when none matches.
	Matches []int
	Chosen  int
}

// Place reads the patch src against the files under root and returns every hunk with its matches,
// and every file the patch leaves, by its path after the patch, with each hunk applied at its chosen
// match as git apply applies it. A file's later hunks are not placed once one of its hunks matches
// nowhere. The error is for a patch or a file it cannot read.
func Place(root string, src []byte) (hunks []Hunk, files map[string]string, err error) {
	diffs, err := parseDiff(src)
	if err != nil {
		return nil, nil, err
	}
	images := map[string][]imageLine{}
	for _, f := range diffs {
		var img []imageLine
		if f.oldPath != "" {
			var ok bool
			if img, ok = images[f.oldPath]; !ok {
				img, err = readImage(root, f.oldPath)
				if err != nil {
					return nil, nil, err
				}
			}
		}
		target := f.oldPath
		if target == "" {
			target = f.newPath
		}
		for n, h := range f.hunks {
			at, matches := place(img, h)
			found := Hunk{Path: target, Number: n + 1, Header: h.header}
			for _, pos := range matches {
				found.Matches = append(found.Matches, pos+1)
			}
			if len(matches) > 0 {
				found.Chosen = at + 1
			}
			hunks = append(hunks, found)
			if len(matches) == 0 {
				// Nothing is left to read the file's later hunks against.
				break
			}
			img = splice(img, at, h)
		}
		if f.oldPath != "" && f.oldPath != f.newPath && !f.copied {
			delete(images, f.oldPath)
		}
		if f.newPath != "" {
			images[f.newPath] = img
		}
	}
	files = map[string]string{}
	for path, img := range images {
		var b strings.Builder
		for _, line := range img {
			b.WriteString(line.text)
		}
		files[path] = b.String()
	}
	return hunks, files, nil
}

// Check returns an error naming every hunk of the patch src whose preimage matches at more than one
// place in the file it targets under root, so git apply could put it somewhere other than its author
// meant. A nil error means every hunk has exactly one place. A hunk with no place at all is named
// too, though git apply --check, run first, refuses such a patch, so naming one means this package
// reads the patch or the file differently from git.
func Check(root string, src []byte) error {
	hunks, _, err := Place(root, src)
	if err != nil {
		return err
	}
	var problems []string
	for _, h := range hunks {
		if len(h.Matches) == 1 {
			continue
		}
		if len(h.Matches) == 0 {
			problems = append(problems, fmt.Sprintf("%s: hunk %d (%s) matches nowhere, which git apply --check would have refused", h.Path, h.Number, h.Header))
			continue
		}
		lines := make([]string, len(h.Matches))
		for i, m := range h.Matches {
			lines[i] = strconv.Itoa(m)
		}
		problems = append(problems, fmt.Sprintf("%s: hunk %d (%s) matches at lines %s, so git apply could place it at any of them; regenerate the patch with enough context to match once", h.Path, h.Number, h.Header, strings.Join(lines, ", ")))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// readImage reads a file under root into lines, each with its line ending.
func readImage(root, path string) ([]imageLine, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s, which the patch changes, does not exist", path)
	}
	if err != nil {
		return nil, err
	}
	var img []imageLine
	for line := range strings.Lines(string(data)) {
		img = append(img, imageLine{text: line})
	}
	return img, nil
}

// place returns every line, counted from 0, at which the hunk's preimage matches img under git
// apply's rules, and the one of them git apply picks, the match nearest the hunk's header line,
// after it when two are as near.
func place(img []imageLine, h hunk) (at int, matches []int) {
	matchBeginning := h.oldStart <= 1
	matchEnd := h.trailing == 0
	for pos := 0; pos+len(h.pre) <= len(img); pos++ {
		if matchBeginning && pos != 0 || matchEnd && pos+len(h.pre) != len(img) {
			continue
		}
		if matchesAt(img, pos, h.pre, matchEnd) {
			matches = append(matches, pos)
		}
	}
	if len(matches) == 0 {
		return 0, nil
	}
	start := max(h.newStart-1, 0)
	at = matches[0]
	for _, pos := range matches[1:] {
		if d, best := distance(pos, start), distance(at, start); d < best || d == best && pos > at {
			at = pos
		}
	}
	return at, matches
}

// matchesAt reports whether pre matches img from line pos. A hunk that must match at the end holds
// a last line without a newline to the file's own last line, which the caller has placed at pos, so
// only a hunk that need not compares that line as a prefix.
func matchesAt(img []imageLine, pos int, pre []string, matchEnd bool) bool {
	for i, text := range pre {
		line := img[pos+i]
		if line.patched {
			return false
		}
		if line.text == text {
			continue
		}
		last := i == len(pre)-1
		if last && !matchEnd && !strings.HasSuffix(text, "\n") && strings.HasPrefix(line.text, text) && onlySpace(line.text[len(text):]) {
			continue
		}
		return false
	}
	return true
}

// onlySpace reports whether s holds nothing but what C's isspace counts as whitespace, which is what
// git's line hash skips.
func onlySpace(s string) bool {
	return strings.Trim(s, " \t\n\v\f\r") == ""
}

func distance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// splice replaces the hunk's preimage at pos with its postimage, marking the written lines.
func splice(img []imageLine, pos int, h hunk) []imageLine {
	out := make([]imageLine, 0, len(img)-len(h.pre)+len(h.post))
	out = append(out, img[:pos]...)
	for _, text := range h.post {
		out = append(out, imageLine{text: text, patched: true})
	}
	return append(out, img[pos+len(h.pre):]...)
}
