package index

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Delisted returns the domains among restricted, the sender domains of the messages stored as
// restricted or skipped as restricted, that the account policy p no longer restricts, in the order
// given (ADR-0037). A domain the classifier cannot read stays restricted, and a policy that never
// loaded restricts every domain, so neither delists anything.
func Delisted(p policy.Composed, l classify.Lookups, restricted []string) []string {
	var out []string
	for _, d := range restricted {
		if !classify.Classify(p, "@"+d, l).Class().Restricted() {
			out = append(out, d)
		}
	}
	return out
}

// Listing is one domain the index stores as normal that the policy now restricts, with the rule that
// restricts it.
type Listing struct {
	Domain, Rule string
}

// Listed returns the domains among normal, the sender domains of the messages stored as normal, that
// the account policy p now restricts under a rule, each with that rule, in the order given (ADR-0113).
// A domain the classifier cannot read, and every domain while no policy has loaded, are restricted
// with no rule, so neither is listed.
func Listed(p policy.Composed, l classify.Lookups, normal []string) []Listing {
	var out []Listing
	for _, d := range normal {
		if v := classify.Classify(p, "@"+d, l); v.Reason() == classify.Listed {
			out = append(out, Listing{Domain: d, Rule: v.Rule()})
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
func Gate(p policy.Composed, l classify.Lookups, c scangate.Config, now mail.UnixMilli, m Waiting, pageHits int64) scangate.Verdict {
	local := m.From
	if at := strings.LastIndexByte(m.From, '@'); at >= 0 {
		local = m.From[:at]
	}
	return scangate.Decide(c, now, scangate.Input{
		Class:         classify.Classify(p, m.From, l).Class(),
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

// Scan returns what a body records once scanned by s, from the Markdown of its HTML part and its text
// part. A flag in either flags the message, since release may serve either (ADR-0017). The rules are
// the content rules alone, each once, sorted. A scanner nobody built flags the body, as its verdict
// does.
func Scan(s scan.Scanner, markdown, text string) Scanned {
	a, b := s.Scan(markdown), s.Scan(text)
	rules := append(append([]string{}, a.Rules()...), b.Rules()...)
	slices.Sort(rules)
	return Scanned{
		Flags:    sensitivity.Flags(a.Flags().MFACode() || b.Flags().MFACode(), a.Flags().LoginLink() || b.Flags().LoginLink()),
		Rules:    slices.Compact(rules),
		Version:  a.Version(),
		Revision: a.Revision(),
	}
}

// Outcome is what a workload did with one waiting message.
type Outcome struct {
	ID, Domain string
	// Verdict is the gate's decision.
	Verdict scangate.Verdict
	// Scanned is what the message's scan records, nil when it was not scanned.
	Scanned *Scanned
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
