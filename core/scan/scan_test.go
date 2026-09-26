package scan_test

import (
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// outcome is what a test can observe of a verdict.
type outcome struct {
	MFACode, LoginLink bool
	Rules              []string
	Tier               int
	Version, Revision  int
}

func observe(v scan.Verdict) outcome {
	return outcome{
		MFACode:   v.Flags().MFACode(),
		LoginLink: v.Flags().LoginLink(),
		Rules:     v.Rules(),
		Tier:      v.Tier(),
		Version:   v.Version(),
		Revision:  v.Revision(),
	}
}

func scanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func code(tier int, rules ...string) outcome {
	return outcome{MFACode: true, Rules: rules, Tier: tier, Version: scan.Version, Revision: 1}
}

func link(rules ...string) outcome {
	return outcome{LoginLink: true, Rules: rules, Tier: 1, Version: scan.Version, Revision: 1}
}

func clean(tier int) outcome { return outcome{Tier: tier, Version: scan.Version, Revision: 1} }

// The paths production never exercises, tested first (ADR-0042). A scanner nobody built returns the
// zero verdict, which carries both flags, and says so to a caller masking with it.
func TestFailClosed(t *testing.T) {
	var zero scan.Scanner
	got := observe(zero.Scan("Nothing to see here."))
	if diff := cmp.Diff(outcome{MFACode: true, LoginLink: true}, got, compare.Options); diff != "" {
		t.Errorf("zero Scanner (-want +got):\n%s", diff)
	}
	if _, ok := zero.SubjectSpans("Nothing"); ok {
		t.Error("a zero Scanner's SubjectSpans reported a scan")
	}
	if diff := cmp.Diff(outcome{MFACode: true, LoginLink: true}, observe(zero.ScanPatterns("Nothing to see here.")), compare.Options); diff != "" {
		t.Errorf("zero Scanner's ScanPatterns (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(outcome{MFACode: true, LoginLink: true}, observe(scan.Verdict{}), compare.Options); diff != "" {
		t.Errorf("zero Verdict (-want +got):\n%s", diff)
	}
}

// Every problem in a configuration is refused, and the refusal returns a scanner that refuses to
// decide.
func TestNewRefusesAnInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		change func(*scan.Config)
		want   string
	}{
		{"a revision below 1", func(c *scan.Config) { c.Revision = 0 }, "the revision is below 1"},
		{"no trigger word", func(c *scan.Config) { c.Triggers = nil }, "no trigger word is configured"},
		{"an empty trigger", func(c *scan.Config) { c.Triggers["en"] = []string{""} }, `the en trigger "" is empty`},
		{"a trigger with surrounding space", func(c *scan.Config) { c.Triggers["en"] = []string{" code"} }, `" code" is empty or has surrounding space`},
		{"no link word", func(c *scan.Config) { c.LinkWords = nil }, "no link word or no link parameter"},
		{"a blank link parameter", func(c *scan.Config) { c.LinkParams = []string{"t", " "} }, `" " is empty`},
		{"a zero window", func(c *scan.Config) { c.Window = 0 }, "must be positive"},
		{"a zero dense length", func(c *scan.Config) { c.DenseLength = 0 }, "must be positive"},
		{"a negative weight", func(c *scan.Config) { c.Weights.Mix = -1 }, "the weights must be"},
		{"all weights zero", func(c *scan.Config) { c.Weights = scan.Weights{} }, "the weights must be"},
		{"an infinite weight", func(c *scan.Config) { c.Weights.Entropy = math.Inf(1) }, "the weights must be"},
		{"a weight that is not a number", func(c *scan.Config) { c.Weights.Length = math.NaN() }, "the weights must be"},
		{"weights summing past the largest number", func(c *scan.Config) { c.Weights.Mix, c.Weights.Length = math.MaxFloat64, math.MaxFloat64 }, "sum to a finite number"},
		{"a threshold of zero", func(c *scan.Config) { c.Threshold = 0 }, "each threshold must be"},
		{"a threshold above 1", func(c *scan.Config) { c.Threshold = 1.5 }, "each threshold must be"},
		{"a threshold that is not a number", func(c *scan.Config) { c.Threshold = math.NaN() }, "each threshold must be"},
		{"a subject threshold of zero", func(c *scan.Config) { c.SubjectThreshold = 0 }, "each threshold must be"},
		{"a subject threshold above the body's", func(c *scan.Config) { c.SubjectThreshold = 0.99 }, "the subject threshold is above the body threshold"},
		{"a dense entropy that is not a number", func(c *scan.Config) { c.DenseEntropy = math.NaN() }, "must be positive"},
		{"a dense entropy no value can reach", func(c *scan.Config) { c.DenseEntropy = 8.5 }, "more than any value of bytes can reach"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := scan.DefaultConfig()
			c.change(&cfg)
			s, err := scan.New(cfg)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("New returned %v, want an error containing %q", err, c.want)
			}
			if !observe(s.Scan("Your code is 419283")).LoginLink {
				t.Error("a refused configuration returned a scanner that decides")
			}
		})
	}
}

// ADR-0005's tier 1 rules for one-time codes, each with the near misses it must not flag.
func TestTier1OneTimeCodes(t *testing.T) {
	s := scanner(t)
	cases := []struct {
		name, body string
		want       outcome
	}{
		{"a code after a trigger", "Your verification code is 419283.", code(1, scan.RuleTriggerWindow)},
		{"a code before a trigger", "419283 is your one-time passcode.", code(1, scan.RuleTriggerWindow)},
		{"a two-word trigger", "Enter the security code 4192.", code(1, scan.RuleTriggerWindow)},
		{"a trigger in capitals", "Your OTP is 41928374.", code(1, scan.RuleTriggerWindow)},
		{"a code in groups of three", "Your code is 419 283.", code(1, scan.RuleTriggerWindow)},
		{"a code joined by a hyphen", "Your code is 419-283.", code(1, scan.RuleTriggerWindow)},
		{"a code in groups joined by a no-break space", "Your code is 419\u00a0283.", code(1, scan.RuleTriggerWindow)},
		{"a code in groups joined by a narrow no-break space", "Your code is 419\u202f283.", code(1, scan.RuleTriggerWindow)},
		{"a code in groups joined by a thin space", "Your code is 419\u2009283.", code(1, scan.RuleTriggerWindow)},
		{"a trigger exactly the window away", "Code one two three four five six seven 419283.", code(1, scan.RuleTriggerWindow)},
		{"a trigger one past the window", "Code one two three four five six seven eight 419283.", clean(2)},
		{"a trigger inside a longer word", "Scan the barcode 419283.", clean(2)},
		{"three digits", "Your code is 419.", clean(1)},
		{"nine digits, which only tier 2 catches", "Your code is 419283746.", code(2, scan.RuleScore)},
		{"digits joined to letters", "Your code is A419283.", code(2, scan.RuleScore)},
		{"a code alone on a line", "Welcome back\n\n419283\n", code(1, scan.RuleOwnLine)},
		{"a code alone on a line in bold", "Welcome back\n\n**419283**\n", code(1, scan.RuleOwnLine)},
		{"a code in a heading", "# Welcome back 419283", code(1, scan.RuleHeadingOrCell)},
		{"a code in a second-level heading", "## 419283", code(1, scan.RuleHeadingOrCell)},
		{"a code in a third-level heading", "### Order 419283", clean(2)},
		{"a code as a table cell", "| Yours | 419283 |\n| --- | --- |", code(1, scan.RuleHeadingOrCell)},
		{"a number inside a table cell's text", "| Order 419283 shipped |", clean(2)},
		{"a code after a no-break space", "Your code is\u00a0419283.", code(1, scan.RuleTriggerWindow)},
		{"a code before a dash", "Your code is 419283\u2014valid for 10 minutes.", code(1, scan.RuleTriggerWindow)},
		{"a code in curly quotes", "Your code is \u201c419283\u201d.", code(1, scan.RuleTriggerWindow)},
		{"a code joined to a letter after it", "Your code is 419283A.", code(2, scan.RuleScore)},
		{"a code alone on a line ending in a carriage return", "Welcome back\r\n\r\n419283\r\n", code(1, scan.RuleOwnLine)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.Scan(c.body)), compare.Options); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", c.body, diff)
			}
		})
	}
}

// ADR-0005's tier 1 rules for login links.
func TestTier1LoginLinks(t *testing.T) {
	s := scanner(t)
	cases := []struct {
		name, body string
		want       outcome
	}{
		{"a dense path segment with a link word", "Go to https://x.example/magic/Q7fT2kLm9ZpX4vRw now.", link(scan.RuleLinkPath)},
		{"a dense path segment without a link word", "Go to https://x.example/blog/Q7fT2kLm9ZpX4vRw now.", clean(1)},
		{"a link word without a dense segment", "Go to https://x.example/auth/login now.", clean(1)},
		{"a dense token parameter", "Go to https://x.example/in?t=h3J9dK2mQ8xR5tY1vB7n now.", link(scan.RuleLinkQuery)},
		{"a parameter name in capitals", "Go to https://x.example/in?TOKEN=h3J9dK2mQ8xR5tY1vB7n now.", link(scan.RuleLinkQuery)}, // gitleaks:allow, a synthetic value
		{"a short token parameter", "Go to https://x.example/in?token=abc123 now.", clean(1)},
		{"a long parameter of another name", "Go to https://x.example/in?page=h3J9dK2mQ8xR5tY1vB7n now.", clean(1)},
		{"a long parameter that is not dense", "Go to https://x.example/in?key=aaaaaaaaaaaaaaaaaaaa now.", clean(1)},
		{"a link inside Markdown", "[Sign in](https://x.example/reset/Q7fT2kLm9ZpX4vRw)", link(scan.RuleLinkPath)},
		{"a link word inside a longer path word", "Go to https://x.example/confirmation/Q7fT2kLm9ZpX4vRw now.", link(scan.RuleLinkPath)},
		{"a link word inside a joined path word", "Go to https://x.example/resetPassword/Q7fT2kLm9ZpX4vRw now.", link(scan.RuleLinkPath)},
		{"a link word only in the fragment", "Go to https://x.example/blog/Q7fT2kLm9ZpX4vRw#magic now.", clean(1)},
		{"a link word only in the host", "Go to https://auth.example/blog/Q7fT2kLm9ZpX4vRw now.", clean(1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.Scan(c.body)), compare.Options); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", c.body, diff)
			}
		})
	}
}

// The serve-time pattern check runs tier 1 alone (ADR-0002). It flags what tier 1's patterns match
// and nothing only tier 2's scoring would, and reports tier 1 as the highest tier evaluated.
func TestScanPatterns(t *testing.T) {
	s := scanner(t)
	cases := []struct {
		name, body string
		want       outcome
	}{
		{"the one-time code fixture", fixture.OneTimeCode().Body, code(1, scan.RuleHeadingOrCell, scan.RuleTriggerWindow)},
		{"the login link fixture", fixture.LoginLink().Body, link(scan.RuleLinkPath, scan.RuleLinkQuery)},
		{"a code alone on a line", "Welcome back\n\n419283\n", code(1, scan.RuleOwnLine)},
		{"the alphanumeric code fixture, which only tier 2 catches", fixture.AlphanumericCode().Body, clean(1)},
		{"nine digits after a trigger, which only tier 2 catches", "Your code is 419283746.", clean(1)},
		{"the receipt fixture", fixture.Receipt().Body, clean(1)},
		{"the newsletter fixture", fixture.Newsletter().Body, clean(1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.ScanPatterns(c.body)), compare.Options); diff != "" {
				t.Errorf("ScanPatterns(%q) (-want +got):\n%s", c.body, diff)
			}
		})
	}
}

// Tier 2 flags what tier 1's patterns miss and leaves ordinary numbers alone.
func TestTier2(t *testing.T) {
	s := scanner(t)
	cases := []struct {
		name, body string
		want       outcome
	}{
		{"an alphanumeric code after a trigger", "Your sign-in code is 7GX4Q2.", code(2, scan.RuleScore)},
		{"an alphanumeric code alone in bold", "Use this code\n\n**7GX4Q2**\n", code(2, scan.RuleScore)},
		{"an order number", "Order A1B2C3D4 shipped today.", clean(2)},
		{"an eleven-character reference after a trigger", "Your code is A1B2C3D4E5F.", clean(1)},
		{"a date", "Delivered on 2024-09-17.", clean(2)},
		{"a word with no digit", "Your verification is complete.", clean(1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.Scan(c.body)), compare.Options); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", c.body, diff)
			}
		})
	}
}

// The subject threshold replaces the body threshold on a subject and nowhere else. A reference with no
// trigger near it scores 0.53, between the two thresholds this configuration sets.
func TestSubjectThreshold(t *testing.T) {
	cfg := scan.DefaultConfig()
	cfg.SubjectThreshold = 0.5
	s, err := scan.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	const text = "Ref 7GX4Q2 is ready"
	if spans, _ := s.SubjectSpans(text); len(spans) != 1 || spans[0].Rule != scan.RuleScore {
		t.Errorf("SubjectSpans(%q) = %+v, want one span from %s", text, spans, scan.RuleScore)
	}
	if diff := cmp.Diff(clean(2), observe(s.Scan(text)), compare.Options); diff != "" {
		t.Errorf("Scan(%q) (-want +got):\n%s", text, diff)
	}
}

// The shared synthetic fixtures.
func TestFixtures(t *testing.T) {
	s := scanner(t)
	cases := []struct {
		name string
		msg  fixture.Message
		want outcome
	}{
		{"bank", fixture.Bank(), clean(1)},
		{"newsletter", fixture.Newsletter(), clean(1)},
		{"one-time code", fixture.OneTimeCode(), code(1, scan.RuleHeadingOrCell, scan.RuleTriggerWindow)},
		{"alphanumeric code", fixture.AlphanumericCode(), code(2, scan.RuleScore)},
		{"login link", fixture.LoginLink(), link(scan.RuleLinkPath, scan.RuleLinkQuery)},
		{"receipt", fixture.Receipt(), clean(2)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.Scan(c.msg.Body)), compare.Options); diff != "" {
				t.Errorf("Scan (-want +got):\n%s", diff)
			}
		})
	}
}

// A verdict holds its flags, the rules that fired, the tier, the version and the revision, and no
// text, and every string it carries is a rule identifier (ADR-0009).
func TestTheVerdictCarriesNoText(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/core/scan", "Verdict",
		"flags sensitivity.ContentFlags", "rules []string", "tier int", "version int", "revision int")
	s := scanner(t)
	for _, m := range fixture.All() {
		for _, r := range s.Scan(m.Body).Rules() {
			if !known[r] || strings.Contains(r, marker.BodyPrefix) {
				t.Errorf("a verdict on %q carries %q, which is not a rule identifier", m.Subject, r)
			}
		}
	}
}

var known = map[string]bool{
	scan.RuleTriggerWindow: true, scan.RuleOwnLine: true, scan.RuleHeadingOrCell: true,
	scan.RuleScore: true, scan.RuleLinkPath: true, scan.RuleLinkQuery: true,
}

// Scanning stays linear in a body's length, so a large body sent to the scanner cannot stall it.
// Eight times the text takes about eight times as long, and an implementation growing with the square
// of the length takes about sixty-four times as long. The bound of sixteen tells them apart without
// depending on how fast the machine is.
func TestScanningStaysLinear(t *testing.T) {
	s := scanner(t)
	unit := "code code A1B2C3\n"
	perScan := func(repeat int) float64 {
		body := strings.Repeat(unit, repeat)
		return float64(testing.Benchmark(func(b *testing.B) {
			for range b.N {
				s.Scan(body)
			}
		}).NsPerOp())
	}
	if ratio := perScan(32000) / perScan(4000); ratio > 16 {
		t.Errorf("eight times the text took %.1f times as long, want under 16", ratio)
	}
}
