// Package scan holds the Content Scanner's two deterministic tiers and the verdict they produce
// (ADR-0005, ADR-0009). The scanner reads a body as the Markdown the sanitizing converter produces,
// which the shell passes in, so the structure the rules look at is Markdown's, a heading as a line
// starting with # or ##, a table row as a line starting with |, and bold as ** or __.
//
// Tier 1 is structural patterns. A one-time code is a 4 to 8 digit run within the token window of a
// trigger word, a line holding only such a run, or such a run in a heading or as a table cell's
// whole content. A login link is a URL with a high-entropy path segment and a link word, or a URL
// query parameter from the configured names whose value is long and dense. Tier 2 scores the
// alphanumeric spans tier 1 did not settle, on entropy, character mix, length, distance to a
// trigger and position, and flags a one-time code at or above the threshold.
//
// The trigger words, the link words and parameter names, the window and tier 2's weights and
// threshold are configuration the shell passes in, config.Scanner, whose defaults are the values the
// application ships. A verdict carries the content flags, the rules that fired, the tier reached,
// the scanner's version and the configuration's revision, and no text, so the body it was made from
// cannot travel in it. A Scanner nobody built by New returns the zero Verdict, which carries both
// flags and withholds the body.
package scan

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Version is the scanner code's version, stamped on every verdict. It changes whenever a rule's
// code changes, so the verdicts an earlier version produced are scanned again.
const Version = 1

// Rule identifiers, recorded on a verdict and on a masking event in place of the matched text.
const (
	RuleTriggerWindow = "mfa.trigger_window"
	RuleOwnLine       = "mfa.own_line"
	RuleHeadingOrCell = "mfa.heading_or_cell"
	RuleScore         = "mfa.score"
	RuleLinkPath      = "link.path_token"
	RuleLinkQuery     = "link.query_token"
)

// Scanner applies both tiers under one configuration. A Scanner is built by New and never changes.
// The zero value refuses to decide.
type Scanner struct {
	built      bool
	patterns   bool
	cfg        config.Scanner
	revision   string
	triggers   automaton
	linkWords  automaton
	linkParams map[string]bool
	url        *regexp.Regexp
}

// New checks cfg and returns a Scanner applying it under revision, the configuration's revision
// every verdict records, or every problem found, joined. The checks of cfg are config.Scanner's.
func New(cfg config.Scanner, revision string) (Scanner, error) {
	var problems []error
	if revision == "" {
		problems = append(problems, errors.New("scan: the revision is empty"))
	}
	if err := cfg.Check(); err != nil {
		problems = append(problems, err)
	}
	if len(problems) > 0 {
		return Scanner{}, errors.Join(problems...)
	}
	var triggers []string
	for _, lang := range slices.Sorted(maps.Keys(cfg.Triggers)) {
		triggers = append(triggers, cfg.Triggers[lang]...)
	}
	params := map[string]bool{}
	for _, p := range cfg.LinkParams {
		params[strings.ToLower(p)] = true
	}
	return Scanner{
		built:      true,
		cfg:        cfg,
		revision:   revision,
		triggers:   newAutomaton(triggers),
		linkWords:  newAutomaton(cfg.LinkWords),
		linkParams: params,
		url:        regexp.MustCompile(`(?i)https?://[^\s<>()\[\]"']+`),
	}, nil
}

// Verdict is one body's scan result. It holds no text. The zero value carries both content flags,
// so a verdict nobody produced withholds the body.
type Verdict struct {
	flags    sensitivity.ContentFlags
	rules    []string
	tier     int
	version  int
	revision string
}

// Flags returns the content flags the scan found.
func (v Verdict) Flags() sensitivity.ContentFlags { return v.flags }

// Rules returns the identifiers of the rules that fired, each once, sorted.
func (v Verdict) Rules() []string { return slices.Clone(v.rules) }

// Tier returns the highest tier the scan evaluated.
func (v Verdict) Tier() int { return v.tier }

// Version returns the scanner version that produced the verdict.
func (v Verdict) Version() int { return v.version }

// Revision returns the revision of the configuration the verdict was produced under.
func (v Verdict) Revision() string { return v.revision }

// Version returns the scanner version the scanner's verdicts and maskings are made under, Version for
// a Scanner built by New and 0 for one nobody built, which matches no version a built one records.
func (s Scanner) Version() int {
	if !s.built {
		return 0
	}
	return Version
}

// Revision returns the revision of the configuration the scanner was built under, empty for a Scanner
// nobody built.
func (s Scanner) Revision() string { return s.revision }

// ScanPatterns returns the verdict of tier 1 alone on body, a message body as Markdown. It is the
// serve-time pattern check a body released without being scanned passes, which runs the structural
// patterns and never tier 2's scoring (ADR-0002). A Scanner nobody built returns the zero Verdict, as
// Scan does.
func (s Scanner) ScanPatterns(body string) Verdict {
	s.patterns = true
	return s.Scan(body)
}

// Scan returns the verdict on body, a message body as Markdown.
func (s Scanner) Scan(body string) Verdict {
	if !s.built {
		return Verdict{}
	}
	spans, tier := s.detect(body, false)
	var mfa, link bool
	var rules []string
	for _, sp := range spans {
		switch sp.Flag {
		case OneTimeCode:
			mfa = true
		case LoginLink:
			link = true
		default:
			mfa, link = true, true
		}
		rules = append(rules, sp.Rule)
	}
	slices.Sort(rules)
	return Verdict{
		flags:    sensitivity.Flags(mfa, link),
		rules:    slices.Compact(rules),
		tier:     tier,
		version:  Version,
		revision: s.revision,
	}
}
