package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ordinaryLinters is the closed list of linters that stand in for no control. Only these may be named by
// a //nolint directive or by an exclusion rule in .golangci.yaml. Every other linter is treated as one
// standing in for a control, so a linter enabled without being listed here cannot be suppressed at all.
//
// A linter joins this list only when no rule it carries stands in for a control. If one ever does, the
// whole linter leaves the list, and every suppression naming it turns banproof red. banproof also fails
// when a listed linter is named by a want annotation, when a listed linter is not enabled, and when an
// enabled linter is neither listed nor named by a want (roleProblems).
var ordinaryLinters = []string{
	"bodyclose",
	"errorlint",
	"gosec",
	"govet",
	"ineffassign",
	"misspell",
	"revive",
	"staticcheck",
	"unused",
}

// nolintPattern is golangci-lint's own test for a suppression directive, which it applies to a comment's
// text after trimming leading slashes and spaces (pkg/result/processors/nolint_filter.go).
var nolintPattern = regexp.MustCompile(`^nolint( |:|$)`)

// looseDirective matches a comment that reads as a suppression directive in any spelling, after leading
// slashes, asterisks and blanks are trimmed. It covers golangci-lint's directive in spellings the tool
// does not honour today, such as another letter case, a block comment or a tab after the slashes, and
// the in-code directives of the linters standing in for controls, which are exhaustive's //exhaustive:
// comments and forbidigo's //permit: comment. Refusing what a tool does not honour today fails closed, so
// a later release that starts honouring a spelling cannot silence a control.
var looseDirective = regexp.MustCompile(`(?i)^(nolint\b|exhaustive:|permit:)`)

// directiveProblem classifies one comment token. It returns why the comment is refused, or false when the
// comment is not a suppression directive or is a //nolint directive naming only ordinary linters with a
// reason.
//
// The //nolint branch copies golangci-lint's parser step by step. A directive not starting with nolint:,
// or starting with nolint:all, silences every linter. Otherwise the text is cut at the next //, split on
// commas, and each name is trimmed and lowercased, and a name equal to all silences every linter. Where
// that directive sits does not matter here. Above a statement, a block, a function or the package clause
// it covers that whole node, but still only for the linters it names.
func directiveProblem(text string, ordinary []string) (string, bool) {
	if body := strings.TrimLeft(text, "/ "); nolintPattern.MatchString(body) {
		if strings.HasPrefix(body, "nolint:all") || !strings.HasPrefix(body, "nolint:") {
			return "silences every linter, including those standing in for controls", true
		}
		names, reason, _ := strings.Cut(strings.TrimPrefix(body, "nolint:"), "//")
		for item := range strings.SplitSeq(names, ",") {
			name := strings.ToLower(strings.TrimSpace(item))
			if name == "all" {
				return "silences every linter, including those standing in for controls", true
			}
			if !slices.Contains(ordinary, name) {
				return fmt.Sprintf("names %q, which is not on banproof's list of ordinary linters", name), true
			}
		}
		if strings.TrimSpace(reason) == "" {
			return "gives no reason. Write the reason after the linter names, as //nolint:gosec // reason", true
		}
		return "", false
	}
	body := strings.TrimLeft(text, "/* \t")
	if m := looseDirective.FindString(body); m != "" {
		switch strings.ToLower(m) {
		case "exhaustive:":
			return "is exhaustive's own directive, which silences a control", true
		case "permit:":
			return "is forbidigo's own directive, which would silence a control", true
		default:
			return "is a spelling of the suppression directive that golangci-lint does not honour today", true
		}
	}
	return "", false
}

// directiveFindings reports every comment in src that is a refused suppression directive.
func directiveFindings(path string, src []byte, ordinary []string) []finding {
	var out []finding
	for _, c := range comments(src) {
		if why, refused := directiveProblem(c.text, ordinary); refused {
			out = append(out, finding{file: path, line: c.line, tool: toolSuppression, text: fmt.Sprintf("suppression directive %s %s", c.text, why)})
		}
	}
	return out
}
