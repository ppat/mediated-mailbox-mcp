package scan_test

import (
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// args are the parts of one generated body, each drawn on its own (ADR-0069). Every body carries a
// body marker, filler words with no digit in them, and optionally a code sentence and a sign-in
// link.
type args struct {
	Tag          string
	Filler       []string
	Code         bool
	Digits       int
	Link         bool
	LinkPosition int
}

var fillerWords = []string{"hello", "your", "account", "update", "thanks", "team", "please", "review", "the", "latest", "notice"}

func draw(t *rapid.T) args {
	return args{
		Tag:          rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "tag"),
		Filler:       rapid.SliceOfN(rapid.SampledFrom(fillerWords), 0, 12).Draw(t, "filler"),
		Code:         rapid.Bool().Draw(t, "code"),
		Digits:       rapid.IntRange(100000, 999999).Draw(t, "digits"),
		Link:         rapid.Bool().Draw(t, "link"),
		LinkPosition: rapid.IntRange(0, 1).Draw(t, "linkPosition"),
	}
}

func body(a args) string {
	parts := []string{marker.Body(a.Tag), strings.Join(a.Filler, " ")}
	if a.Code {
		parts = append(parts, "Your verification code is "+strconv.Itoa(a.Digits)+".")
	}
	if a.Link {
		l := "[Sign in](https://login.example/auth/magic/Q7fT2kLm9ZpX4vRw)"
		parts = append(parts[:a.LinkPosition+1], append([]string{l}, parts[a.LinkPosition+1:]...)...)
	}
	return strings.Join(parts, "\n\n")
}

// A postcondition over every generated body, of ADR-0009's output rule. Nothing a verdict carries
// contains the body's marker or any other body text, every string in it is a rule identifier, and
// the flags match what the body holds, so a scanner that stops flagging, or flags everything, fails
// too.
func TestNoScannerOutputCarriesBodyText(t *testing.T) {
	s, err := scan.New(scan.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	property.Check(t, draw, func(t rapid.TB, a args) {
		b := body(a)
		v := s.Scan(b)
		for _, r := range v.Rules() {
			if !known[r] || strings.Contains(r, marker.BodyPrefix) || strings.Contains(b, r) {
				t.Fatalf("%+v: the verdict carries %q, which is not a rule identifier kept apart from the body", a, r)
			}
		}
		if v.Flags().MFACode() != a.Code || v.Flags().LoginLink() != a.Link {
			t.Fatalf("%+v: flags one-time code %v and login link %v, want %v and %v",
				a, v.Flags().MFACode(), v.Flags().LoginLink(), a.Code, a.Link)
		}
	})
}

func kind(a args) string {
	switch {
	case a.Code && a.Link:
		return "code and link"
	case a.Code:
		return "code"
	case a.Link:
		return "link"
	default:
		return "neither"
	}
}

// The generator report for the property above. Each kind is one in four, and across thirty seeds at
// 200 cases none fell below 1 percent.
func TestNoScannerOutputCarriesBodyTextMix(t *testing.T) {
	property.Report(t, draw, kind, map[string]float64{
		"code and link": 0.01,
		"code":          0.01,
		"link":          0.01,
		"neither":       0.01,
	})
}
