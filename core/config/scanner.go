package config

import (
	"errors"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Weights weigh the content scanner's tier 2 features, each scored between 0 and 1.
type Weights struct {
	Entropy   float64 `yaml:"entropy"`
	Mix       float64 `yaml:"mix"`
	Length    float64 `yaml:"length"`
	Proximity float64 `yaml:"proximity"`
	Position  float64 `yaml:"position"`
}

// Scanner is the content scanner's part, under the key scanner, which core/scan builds a scanner
// from (ADR-0005). It carries no revision. The revision a verdict records is the configuration
// library's revision of this part, which the program hands the scanner, so a change in any layer
// changes it (ADR-0078). Its YAML rendering is what that revision hashes, so a field added, renamed
// or reordered changes the revision of every deployment (ADR-0120).
type Scanner struct {
	// Triggers are the trigger words for one-time codes, by language.
	Triggers map[string][]string `yaml:"triggers"`
	// LinkWords are the words that, with a high-entropy path segment, make a URL a login link.
	LinkWords []string `yaml:"link_words"`
	// LinkParams are the query parameter names whose long, dense value makes a URL a login link.
	LinkParams []string `yaml:"link_params"`
	// Window is how many tokens may separate a trigger word from a digit run.
	Window int `yaml:"window"`
	// DenseLength and DenseEntropy make a path segment or a parameter value dense, at least that
	// many characters and at least that many bits of Shannon entropy per character.
	DenseLength  int     `yaml:"dense_length"`
	DenseEntropy float64 `yaml:"dense_entropy"`
	// Weights and Threshold are tier 2's. A span scoring at or above the threshold is a one-time
	// code. SubjectThreshold replaces Threshold on a subject, and may not exceed it, because masking a
	// subject is tuned for recall (ADR-0003).
	Weights          Weights `yaml:"weights"`
	Threshold        float64 `yaml:"threshold"`
	SubjectThreshold float64 `yaml:"subject_threshold"`
}

// DefaultScanner returns the scanner part the application ships, ADR-0005's English vocabulary and
// the tuning set against the evaluation set in core/scan's tests, to be retuned against the real
// mailbox. On that set equal weights separate codes from everything else. Every code there that only
// tier 2 can catch has a trigger word near it or stands alone or in bold, and scores at least 0.73,
// and the lowest such code in core/scan's other tests, nine digits after a trigger, scores 0.65.
// Order numbers, flight numbers and product references have neither and score at most 0.53, the
// same as a code with neither would. Both thresholds sit between the two. A subject threshold at or
// below 0.53 would mask alphanumeric references in a subject while catching no code the set holds,
// because every code a subject carries in the templates the set is drawn from has a trigger word
// beside it.
func DefaultScanner() Scanner {
	return Scanner{
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

// Check refuses a scanner part no scanner can decide under, and returns every problem found,
// joined. Every trigger word, link word and parameter name is a word without surrounding space, at
// least one of each is configured, the window, the dense length and the dense entropy are positive,
// the dense entropy at most 8 bits, the weights finite, not negative and not all zero, each
// threshold above 0 and at most 1, and the subject threshold at most the body's.
func (s Scanner) Check() error {
	var problems []error
	triggers := 0
	for _, lang := range slices.Sorted(maps.Keys(s.Triggers)) {
		for _, w := range s.Triggers[lang] {
			if !word(w) {
				problems = append(problems, errors.New("scan: the "+lang+" trigger "+strconv.Quote(w)+" is empty or has surrounding space"))
			}
			triggers++
		}
	}
	if triggers == 0 {
		problems = append(problems, errors.New("scan: no trigger word is configured"))
	}
	for _, w := range slices.Concat(s.LinkWords, s.LinkParams) {
		if !word(w) {
			problems = append(problems, errors.New("scan: the link word or parameter "+strconv.Quote(w)+" is empty or has surrounding space"))
		}
	}
	if len(s.LinkWords) == 0 || len(s.LinkParams) == 0 {
		problems = append(problems, errors.New("scan: no link word or no link parameter is configured"))
	}
	if s.Window < 1 || s.DenseLength < 1 || !(s.DenseEntropy > 0) {
		problems = append(problems, errors.New("scan: the window, the dense length and the dense entropy must be positive"))
	} else if s.DenseEntropy > 8 {
		problems = append(problems, errors.New("scan: the dense entropy is above 8 bits, more than any value of bytes can reach, so no link would be flagged"))
	}
	w := s.Weights
	weights := []float64{w.Entropy, w.Mix, w.Length, w.Proximity, w.Position}
	if slices.ContainsFunc(weights, func(x float64) bool { return !(x >= 0) || math.IsInf(x, 0) }) ||
		sum(weights) == 0 || math.IsInf(sum(weights), 0) {
		problems = append(problems, errors.New("scan: the weights must be finite, not negative, not all zero, and sum to a finite number"))
	}
	if !(s.Threshold > 0 && s.Threshold <= 1) || !(s.SubjectThreshold > 0 && s.SubjectThreshold <= 1) {
		problems = append(problems, errors.New("scan: each threshold must be above 0 and at most 1"))
	} else if s.SubjectThreshold > s.Threshold {
		problems = append(problems, errors.New("scan: the subject threshold is above the body threshold, but masking a subject is tuned for recall"))
	}
	return errors.Join(problems...)
}

func word(w string) bool { return w != "" && strings.TrimSpace(w) == w }

func sum(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s
}
