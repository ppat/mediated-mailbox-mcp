package redact_test

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// subjectArgs are the parts of one generated subject, each drawn on its own (ADR-0069). Every
// subject carries a field marker and one of the ways a one-time code is written into a subject.
type subjectArgs struct {
	Tag    string
	Phrase int
	Digits int
}

var phrases = []func(code string) string{
	func(code string) string { return "Your code is " + code },
	func(code string) string { return code + " is your verification code" },
	func(code string) string { return "Security code " + code + " for your account" },
	func(code string) string { return "OTP: " + code },
}

func drawSubject(t *rapid.T) subjectArgs {
	return subjectArgs{
		Tag:    rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "tag"),
		Phrase: rapid.IntRange(0, len(phrases)-1).Draw(t, "phrase"),
		Digits: rapid.IntRange(1000, 99999999).Draw(t, "digits"),
	}
}

// A postcondition over every subject a one-time code is written into. The code never survives, the
// field marker always does, and the subject keeps its length in characters, so a masker that masks
// nothing, or masks everything, fails (ADR-0003).
func TestMaskingRemovesEveryCode(t *testing.T) {
	s, err := scan.New(scan.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	property.Check(t, drawSubject, func(t rapid.TB, a subjectArgs) {
		code := strconv.Itoa(a.Digits)
		subject := marker.Field(a.Tag) + " " + phrases[a.Phrase](code)
		got := redact.MaskSubject(s, subject).Subject()
		switch {
		case strings.Contains(got, code):
			t.Fatalf("%+v: the code survived masking in %q", a, got)
		case !strings.Contains(got, marker.Field(a.Tag)):
			t.Fatalf("%+v: the field marker did not survive masking in %q", a, got)
		case utf8.RuneCountInString(got) != utf8.RuneCountInString(subject):
			t.Fatalf("%+v: masking changed the length of %q to %q", a, subject, got)
		}
	})
}

func phraseKind(a subjectArgs) string { return "phrase " + strconv.Itoa(a.Phrase) }

// The generator report for the property above. Each phrase is one in four, and across thirty seeds
// at 200 cases none fell below 1 percent.
func TestMaskingRemovesEveryCodeMix(t *testing.T) {
	property.Report(t, drawSubject, phraseKind, map[string]float64{
		"phrase 0": 0.01, "phrase 1": 0.01, "phrase 2": 0.01, "phrase 3": 0.01,
	})
}
