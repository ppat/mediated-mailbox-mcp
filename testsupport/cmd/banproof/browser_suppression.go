package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// browserBanRules is the closed list of oxlint rules that stand in for controls in the browser layer
// (ADR-0063, ADR-0064, ADR-0072). No suppression comment and no configuration may switch one of them off.
// Every other oxlint rule is ordinary. Every ast-grep rule is a ban, so ast-grep needs no list.
//
// Entries are rule names without a plugin prefix, because oxlint honours a suppression comment naming a
// rule under any prefix: // oxlint-disable-next-line @typescript-eslint/no-restricted-properties silenced
// eslint's no-restricted-properties. banproof fails when an entry is wanted by no violation file, and when
// a violation file wants an oxlint rule that is not on the list (banRuleProblems).
var browserBanRules = []string{
	"consistent-type-assertions",
	"no-danger",
	"no-explicit-any",
	"no-require-imports",
	"no-restricted-globals",
	"no-restricted-imports",
	"no-restricted-properties",
}

// oxlintDirective matches a suppression comment in a spelling oxlint honours: the directive word in lower
// case, first in a line comment or a block comment. Group 1 is the comment's opening, group 2 the word.
var oxlintDirective = regexp.MustCompile(`(//|/\*)[ \t]*((?:eslint|oxlint)-disable(?:-next-line|-line)?)(?:[ \t]|\*/|$)`)

// looseBrowserDirective matches the directive words of oxlint and ast-grep in any spelling, anywhere on a
// line, strings included. What remains of a line after its honoured oxlint directives are classified is
// refused when it matches, so a spelling the tools do not honour today fails closed.
var looseBrowserDirective = regexp.MustCompile(`(?i)(eslint|oxlint)-disable|ast-grep-ignore`)

// browserDirectiveProblems classifies the suppression comments on one line, with any want annotation
// already removed. An oxlint directive is allowed when it names only ordinary rules and gives a reason after
// --. A directive naming no rule silences every rule, and one naming a ban rule silences a control. Where it
// sits does not matter here, because a directive covering its line, the next line or the rest of the file
// still covers only the rules it names.
func browserDirectiveProblems(text string, bans []string) []string {
	var problems []string
	rest := text
	for {
		m := oxlintDirective.FindStringSubmatchIndex(rest)
		if m == nil {
			break
		}
		body := rest[m[5]:]
		end := len(body)
		if rest[m[2]:m[3]] == "/*" {
			if i := strings.Index(body, "*/"); i >= 0 {
				end = i
			}
		}
		word := rest[m[4]:m[5]]
		problems = append(problems, oxlintDirectiveProblems(word, body[:end], bans)...)
		// Blank out what was classified, so the loose search sees only the remainder.
		rest = rest[:m[4]] + strings.Repeat(" ", len(word)+end) + body[end:]
	}
	if looseBrowserDirective.MatchString(rest) {
		if strings.Contains(strings.ToLower(rest), "ast-grep-ignore") {
			problems = append(problems, "silences an ast-grep rule, and every ast-grep rule is a ban")
		} else {
			problems = append(problems, "is a spelling of a suppression directive that oxlint does not honour today")
		}
	}
	return problems
}

func oxlintDirectiveProblems(word, body string, bans []string) []string {
	names, reason, _ := strings.Cut(body, "--")
	var rules []string
	for item := range strings.SplitSeq(names, ",") {
		if name := strings.ToLower(strings.TrimSpace(item)); name != "" {
			rules = append(rules, name)
		}
	}
	if len(rules) == 0 {
		return []string{word + " names no rule, so it silences every rule, including the bans"}
	}
	var problems []string
	for _, name := range rules {
		if slices.Contains(bans, name[strings.LastIndex(name, "/")+1:]) {
			problems = append(problems, fmt.Sprintf("%s names %q, which is one of the browser's ban rules", word, name))
		}
	}
	if len(problems) == 0 && strings.TrimSpace(reason) == "" {
		problems = append(problems, word+" gives no reason. Write it after the rule names, as // oxlint-disable-next-line no-debugger -- reason")
	}
	return problems
}

// banRuleProblems cross-checks the list of ban rules against the oxlint rules the violation files want.
// wanted holds the rule name, without its plugin, of every oxlint finding a want paired with.
func banRuleProblems(bans []string, wanted map[string]bool) []string {
	var problems []string
	for _, rule := range bans {
		if !wanted[rule] {
			problems = append(problems, fmt.Sprintf("%s is on banproof's list of browser ban rules but no violation file wants its findings", rule))
		}
	}
	for rule := range wanted {
		if !slices.Contains(bans, rule) {
			problems = append(problems, fmt.Sprintf("a violation file wants oxlint's %s, which is not on banproof's list of browser ban rules", rule))
		}
	}
	slices.Sort(problems)
	return problems
}

// oxlintCode matches the start of an oxlint finding's text, such as eslint(no-restricted-properties).
var oxlintCode = regexp.MustCompile(`^[\w@/-]+\(([\w-]+)\)`)
