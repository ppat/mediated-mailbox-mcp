package index

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Delisted returns the domains among restricted, the sender domains of the messages skipped as
// restricted, that the account policy p no longer restricts, in the order given (ADR-0037). A domain
// the classifier cannot read stays restricted, and a policy that never loaded restricts every domain,
// so neither delists anything.
func Delisted(p classify.Policy, restricted []string) []string {
	var out []string
	for _, d := range restricted {
		if !classify.Classify(p, "@"+d).Class().Restricted() {
			out = append(out, d)
		}
	}
	return out
}

// Waiting is one message waiting for a scan, as a read of the waiting messages returns it, or one the
// gate skipped, as the read of the stored gate skips returns it.
type Waiting struct {
	ID string
	// From is the sender's address, and Domain its domain as the index stores it.
	From, Domain string
	// SubjectMasked reports whether the subject was masked at rest.
	SubjectMasked bool
	// ListID reports whether the message carries its own List-Id header.
	ListID    bool
	SizeBytes int64
	SentAt    mail.UnixMilli
	// SenderVolume and SenderHits are the sender's volume and prior hits as the read found them.
	SenderVolume, SenderHits int64
}

// Gate returns the scan gate's decision on m, with the sender's class under the account policy p and
// its prior hits counting pageHits, the hits found among the sender's messages decided before m in the
// same read, so a sender's first hit reaches its next message (ADR-0094). The thresholds are c, and
// the message's age is measured at now.
func Gate(p classify.Policy, c scangate.Config, now mail.UnixMilli, m Waiting, pageHits int64) scangate.Verdict {
	local := m.From
	if at := strings.LastIndexByte(m.From, '@'); at >= 0 {
		local = m.From[:at]
	}
	return scangate.Decide(c, now, scangate.Input{
		Class:         classify.Classify(p, m.From).Class(),
		SubjectMasked: m.SubjectMasked,
		ListID:        m.ListID,
		LocalPart:     local,
		SizeBytes:     m.SizeBytes,
		SentAt:        m.SentAt,
		SenderVolume:  m.SenderVolume,
		PriorHits:     m.SenderHits + pageHits,
	})
}

// Scanned is what a scanned body records, its content flags, the identifiers of the content rules
// that fired, the scanner version and the configuration's revision (ADR-0009).
type Scanned struct {
	Flags    sensitivity.ContentFlags
	Rules    []string
	Version  int
	Revision string
}

// Scan returns what a body records once scanned by s, from rendered, the Markdown its rendered part
// is released as, so the scan reads exactly what release would serve (ADR-0131). The rules
// are the content rules alone, each once, sorted. A scanner nobody built flags the body, as its
// verdict does.
func Scan(s scan.Scanner, rendered string) Scanned {
	v := s.Scan(rendered)
	rules := append([]string{}, v.Rules()...)
	slices.Sort(rules)
	return Scanned{
		Flags:    sensitivity.Flags(v.Flags().MFACode(), v.Flags().LoginLink()),
		Rules:    slices.Compact(rules),
		Version:  v.Version(),
		Revision: v.Revision(),
	}
}

// Outcome is what a workload did with one waiting message.
type Outcome struct {
	ID, Domain string
	// Verdict is the gate's decision.
	Verdict scangate.Verdict
	// Scanned is what the message's scan records, nil when it was not scanned.
	Scanned *Scanned
	// Refused is set when the message's body was refused for its size or by its conversion, so the
	// message is recorded as refused rather than left waiting (ADR-0132). Scanned is then nil.
	Refused bool
}

// Hits returns how many of the scanned messages among outcomes carry a content flag, by the sender's
// domain, which adds to each sender's prior hits.
func Hits(outcomes []Outcome) map[string]int64 {
	hits := map[string]int64{}
	for _, o := range outcomes {
		if o.Scanned != nil && o.Scanned.Flags.Any() {
			hits[o.Domain]++
		}
	}
	return hits
}
