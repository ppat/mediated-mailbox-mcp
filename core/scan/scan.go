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
// threshold are configuration the shell passes in, and DefaultConfig gives the values the
// application ships. A verdict carries the content flags, the rules that fired, the tier reached,
// the scanner's version and the configuration's revision, and no text, so the body it was made from
// cannot travel in it. A Scanner nobody built by New returns the zero Verdict, which carries both
// flags and withholds the body.
package scan

import (
	"errors"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

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

// Weights weigh tier 2's five features, each scored between 0 and 1.
type Weights struct {
	Entropy, Mix, Length, Proximity, Position float64
}

// Config is the scanner's configuration.
type Config struct {
	// Revision numbers the configuration, and a verdict records it, so a verdict made under an
	// earlier configuration is scanned again.
	Revision int
	// Triggers are the trigger words for one-time codes, by language.
	Triggers map[string][]string
	// LinkWords are the words that, with a high-entropy path segment, make a URL a login link.
	LinkWords []string
	// LinkParams are the query parameter names whose long, dense value makes a URL a login link.
	LinkParams []string
	// Window is how many tokens may separate a trigger word from a digit run.
	Window int
	// DenseLength and DenseEntropy make a path segment or a parameter value dense, at least that
	// many characters and at least that many bits of Shannon entropy per character.
	DenseLength  int
	DenseEntropy float64
	// Weights and Threshold are tier 2's. A span scoring at or above the threshold is a one-time
	// code. SubjectThreshold replaces Threshold on a subject, and may not exceed it, because masking a
	// subject is tuned for recall (ADR-0003).
	Weights          Weights
	Threshold        float64
	SubjectThreshold float64
}

// DefaultConfig returns the configuration the application ships, ADR-0005's English vocabulary and
// the tuning set against the evaluation set in this package's tests, to be retuned against the real
// mailbox. On that set equal weights separate codes from everything else. Every code there that only
// tier 2 can catch has a trigger word near it or stands alone or in bold, and scores at least 0.73,
// and the lowest such code in this package's other tests, nine digits after a trigger, scores 0.65.
// Order numbers, flight numbers and product references have neither and score at most 0.53, the
// same as a code with neither would. Both thresholds sit between the two. A subject threshold at or
// below 0.53 would mask alphanumeric references in a subject while catching no code the set holds,
// because every code a subject carries in the templates the set is drawn from has a trigger word
// beside it.
func DefaultConfig() Config {
	return Config{
		Revision: 1,
		Triggers: map[string][]string{
			"en": {
				"code", "otp", "verification", "verify", "pin", "passcode", "password", "2fa",
				"two-factor", "two factor", "2-factor", "one-time", "one time", "single-use",
				"security code", "login", "log in", "log-in", "sign-in", "sign in", "auth",
				"authenticate", "authentication", "secret", "access", "validate", "validation",
				"tan", "confirmation",
			},
		},
		LinkWords: []string{"token", "confirm", "verify", "reset", "magic", "auth", "password", "login", "unlock"},
		LinkParams: []string{
			"token", "code", "key", "auth", "t", "otp", "reset_password_token", "confirmation_token",
			"unlock_token", "oobcode", "verification_code", "confirmation_code", "ticket", "signature",
		},
		Window:           8,
		DenseLength:      16,
		DenseEntropy:     3.0,
		Weights:          Weights{Entropy: 1, Mix: 1, Length: 1, Proximity: 1, Position: 1},
		Threshold:        0.6,
		SubjectThreshold: 0.6,
	}
}

// Scanner applies both tiers under one configuration. A Scanner is built by New and never changes.
// The zero value refuses to decide.
type Scanner struct {
	built      bool
	cfg        Config
	triggers   automaton
	linkWords  automaton
	linkParams map[string]bool
	url        *regexp.Regexp
}

// New validates cfg and returns a Scanner applying it, or every problem found, joined.
func New(cfg Config) (Scanner, error) {
	var problems []error
	if cfg.Revision < 1 {
		problems = append(problems, errors.New("scan: the revision is below 1"))
	}
	var triggers []string
	for _, lang := range slices.Sorted(maps.Keys(cfg.Triggers)) {
		for _, w := range cfg.Triggers[lang] {
			if !word(w) {
				problems = append(problems, errors.New("scan: the "+lang+" trigger "+strconv.Quote(w)+" is empty or has surrounding space"))
			}
			triggers = append(triggers, w)
		}
	}
	if len(triggers) == 0 {
		problems = append(problems, errors.New("scan: no trigger word is configured"))
	}
	for _, w := range slices.Concat(cfg.LinkWords, cfg.LinkParams) {
		if !word(w) {
			problems = append(problems, errors.New("scan: the link word or parameter "+strconv.Quote(w)+" is empty or has surrounding space"))
		}
	}
	if len(cfg.LinkWords) == 0 || len(cfg.LinkParams) == 0 {
		problems = append(problems, errors.New("scan: no link word or no link parameter is configured"))
	}
	if cfg.Window < 1 || cfg.DenseLength < 1 || !(cfg.DenseEntropy > 0) {
		problems = append(problems, errors.New("scan: the window, the dense length and the dense entropy must be positive"))
	} else if cfg.DenseEntropy > 8 {
		problems = append(problems, errors.New("scan: the dense entropy is above 8 bits, more than any value of bytes can reach, so no link would be flagged"))
	}
	w := cfg.Weights
	weights := []float64{w.Entropy, w.Mix, w.Length, w.Proximity, w.Position}
	if slices.ContainsFunc(weights, func(x float64) bool { return !(x >= 0) || math.IsInf(x, 0) }) ||
		sum(weights) == 0 || math.IsInf(sum(weights), 0) {
		problems = append(problems, errors.New("scan: the weights must be finite, not negative, not all zero, and sum to a finite number"))
	}
	if !(cfg.Threshold > 0 && cfg.Threshold <= 1) || !(cfg.SubjectThreshold > 0 && cfg.SubjectThreshold <= 1) {
		problems = append(problems, errors.New("scan: each threshold must be above 0 and at most 1"))
	} else if cfg.SubjectThreshold > cfg.Threshold {
		problems = append(problems, errors.New("scan: the subject threshold is above the body threshold, but masking a subject is tuned for recall"))
	}
	if len(problems) > 0 {
		return Scanner{}, errors.Join(problems...)
	}
	params := map[string]bool{}
	for _, p := range cfg.LinkParams {
		params[strings.ToLower(p)] = true
	}
	return Scanner{
		built:      true,
		cfg:        cfg,
		triggers:   newAutomaton(triggers),
		linkWords:  newAutomaton(cfg.LinkWords),
		linkParams: params,
		url:        regexp.MustCompile(`(?i)https?://[^\s<>()\[\]"']+`),
	}, nil
}

func word(w string) bool { return w != "" && strings.TrimSpace(w) == w }

func sum(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s
}

// Verdict is one body's scan result. It holds no text. The zero value carries both content flags,
// so a verdict nobody produced withholds the body.
type Verdict struct {
	flags    sensitivity.ContentFlags
	rules    []string
	tier     int
	version  int
	revision int
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
func (v Verdict) Revision() int { return v.revision }

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
		revision: s.cfg.Revision,
	}
}
