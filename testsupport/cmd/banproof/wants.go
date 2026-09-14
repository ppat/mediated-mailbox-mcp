package main

import (
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// A want is one finding a violation file expects. It is read from a comment on the line where the
// finding is reported. The comment's text is the word want followed by one or more pairs of a tool name
// and a quoted regular expression that the finding's message must match. The tool name is the
// golangci-lint linter's name, or suppression for the search for suppression directives.
type want struct {
	file string
	line int
	tool string
	re   *regexp.Regexp
}

func (w want) String() string {
	return fmt.Sprintf("%s:%d: %s %q", w.file, w.line, w.tool, w.re)
}

// A comment is one comment token with the line it starts on.
type comment struct {
	line int
	text string
}

// comments returns every comment in a Go source file. It tokenizes rather than searching the text,
// so a string literal that looks like a comment is not taken for one.
func comments(src []byte) []comment {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	var out []comment
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.COMMENT {
			out = append(out, comment{line: file.Line(pos), text: lit})
		}
	}
}

// parseWants reads the want annotations of one file.
func parseWants(path string, src []byte) ([]want, error) {
	var out []want
	for _, c := range comments(src) {
		body, ok := strings.CutPrefix(c.text, "//")
		if !ok {
			// A block comment holds an annotation when that is all it holds. It lets a want share a line
			// with a directive whose spelling a line comment's annotation cannot follow, such as // nolint.
			block := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(c.text, "/*"), "*/"))
			if rest, isWant := strings.CutPrefix(block, "want "); isWant {
				pairs, err := parseWantPairs(rest)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: %w", path, c.line, err)
				}
				for _, p := range pairs {
					out = append(out, want{file: path, line: c.line, tool: p.tool, re: p.re})
				}
			}
			continue
		}
		// A want may follow a directive in the same line comment, as on a line proving the
		// suppression search. A directive has no space after the slashes, which tells it apart
		// from prose that mentions an annotation.
		if !strings.HasPrefix(body, " ") && !strings.HasPrefix(body, "\t") {
			if _, after, found := strings.Cut(body, " // want"); found {
				body = "want" + after
			}
		}
		body = strings.TrimSpace(body)
		rest, ok := strings.CutPrefix(body, "want ")
		if !ok && body != "want" {
			continue
		}
		pairs, err := parseWantPairs(rest)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, c.line, err)
		}
		for _, p := range pairs {
			out = append(out, want{file: path, line: c.line, tool: p.tool, re: p.re})
		}
	}
	return out, nil
}

type wantPair struct {
	tool string
	re   *regexp.Regexp
}

func parseWantPairs(s string) ([]wantPair, error) {
	var out []wantPair
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		end := strings.IndexFunc(s, unicode.IsSpace)
		if end <= 0 {
			return nil, fmt.Errorf("want annotation %q has a tool with no pattern", s)
		}
		tool := s[:end]
		s = strings.TrimSpace(s[end:])
		quoted, err := strconv.QuotedPrefix(s)
		if err != nil {
			return nil, fmt.Errorf("want annotation for %s needs a quoted pattern: %w", tool, err)
		}
		pattern, err := strconv.Unquote(quoted)
		if err != nil {
			return nil, fmt.Errorf("want annotation for %s: %w", tool, err)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("want annotation for %s: %w", tool, err)
		}
		out = append(out, wantPair{tool: tool, re: re})
		s = s[len(quoted):]
	}
	if len(out) == 0 {
		return nil, errors.New("want annotation names no finding")
	}
	return out, nil
}
