package redact

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
)

// maskRune replaces each character of a masked span.
const maskRune = '█'

// RuleWholeSubject is the rule an event names when the whole subject was masked because no scanner
// was built.
const RuleWholeSubject = "mask.whole_subject"

// Event is one mask applied to a subject. It names the rule and tier that detected what was
// masked, never the text (ADR-0003).
type Event struct {
	rule string
	tier int
}

// Rule returns the identifier of the rule that detected what was masked.
func (e Event) Rule() string { return e.rule }

// Tier returns the tier of that rule, 0 for the whole subject masked without a scanner.
func (e Event) Tier() int { return e.tier }

// Masked is a subject after masking, with an event for each detection. The zero value is an empty
// subject.
type Masked struct {
	subject string
	events  []Event
}

// Subject returns the masked subject, the form the index stores and a client receives.
func (m Masked) Subject() string { return m.subject }

// Events returns one event for each detection masked. Two rules detecting the same characters give
// two events.
func (m Masked) Events() []Event { return slices.Clone(m.events) }

// MaskSubject masks every one-time code and login link the scanner detects in subject under subject
// masking's tuning, replacing each character of each detection with █. It takes no sender class,
// because masking runs on every message, a restricted sender's included (ADR-0003). A Scanner nobody
// built by scan.New masks the whole subject.
func MaskSubject(s scan.Scanner, subject string) Masked {
	spans, ok := s.SubjectSpans(subject)
	if !ok {
		return Masked{
			subject: strings.Repeat(string(maskRune), len([]rune(subject))),
			events:  []Event{{rule: RuleWholeSubject}},
		}
	}
	masked := []byte(subject)
	hidden := make([]bool, len(subject))
	var events []Event
	for _, sp := range spans {
		for i := sp.Start; i < sp.End; i++ {
			hidden[i] = true
		}
		events = append(events, Event{rule: sp.Rule, tier: sp.Tier})
	}
	var b strings.Builder
	for i := 0; i < len(masked); {
		if !hidden[i] {
			b.WriteByte(masked[i])
			i++
			continue
		}
		b.WriteRune(maskRune)
		i++
		for i < len(masked) && hidden[i] && masked[i]&0xC0 == 0x80 {
			i++
		}
	}
	return Masked{subject: b.String(), events: events}
}
