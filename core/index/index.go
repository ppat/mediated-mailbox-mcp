// Package index holds the decisions every workload that writes the metadata index makes about a
// message, as values (ADR-0040). Backfill and delta sync both write the index, and each must decide
// a message the way the other does, or each reopens the other's work (ADR-0096, ADR-0098), so the
// decisions sit here once.
//
// Decide turns provider metadata into the rows the index stores, each sender classified against the
// account's policy and each subject masked by the scanner, a restricted sender's included (ADR-0003,
// ADR-0004). Delisted is the delisting transition's comparison (ADR-0037), and Listed its counterpart
// for an added rule (ADR-0113). Gate decides whether one waiting message's body is scanned, from its
// own inputs and its sender's current ones (ADR-0093, ADR-0094). Scan decides what a scanned body
// records. ClassOf names the error class a run records for a failure the provider reported
// (ADR-0016).
package index

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
)

// Stamp is the scanner version and configuration revision a subject's masking or a body's scan is
// made under (ADR-0009, ADR-0096).
type Stamp struct {
	Version  int
	Revision string
}

// StampOf returns the stamp of what s decides. A scanner nobody built has a stamp no built one has.
func StampOf(s scan.Scanner) Stamp {
	return Stamp{Version: s.Version(), Revision: s.Revision()}
}

// Class is a sender's class as the index stores it.
type Class string

const (
	// Normal is a sender no rule lists.
	Normal Class = "normal"
	// Restricted is a sender a rule lists, or one that cannot be classified (ADR-0004).
	Restricted Class = "restricted"
)

// Message is one message's row as the index stores it. It holds no body, snippet or attachment name
// (ADR-0016).
type Message struct {
	ID, ThreadID string
	From         mail.Address
	// Domain is the part of the sender's address after its last @, in the form StoredDomain gives
	// it, or empty for an address without one.
	Domain string
	// Subject is the subject masked at rest (ADR-0003).
	Subject        string
	SubjectMasked  bool
	Date           mail.UnixMilli
	Labels         []string
	Flags          mail.Flags
	HasAttachments bool
	ListID         string
	SizeBytes      int64
	AuthResults    mail.AuthResults
	Class          Class
	// ClassRule is the identifier of the policy rule that set the class, empty when no rule set it,
	// which is a sender no rule lists, a classification made while no policy has loaded and an
	// address that cannot be classified (ADR-0016).
	ClassRule string
	// Unclassified is set when the classifier could not classify the sender, which classifies it
	// restricted.
	Unclassified bool
	// Masks are the masks applied to the subject, one per detection.
	Masks []Mask
	// Stamp is the scanner the subject was masked under (ADR-0096).
	Stamp Stamp
}

// Mask is one mask applied to a subject, naming the rule and tier that detected what was masked and
// never the text (ADR-0003).
type Mask struct {
	Rule string
	Tier int
}

// Page is what one set of metadata adds to the index.
type Page struct {
	Messages []Message
	// Domains are the senders' domains the page holds, each once, sorted, whose statistics the shell
	// rebuilds.
	Domains []string
}

// Decide returns the rows a set of metadata adds to the index, each sender classified under the
// account's policy p and each subject masked by s under subject masking's tuning and stamped with s. A
// policy that never loaded restricts every sender, and a scanner nobody built masks every subject
// whole, so neither fails open.
func Decide(items []mail.MessageMetadata, p policy.Composed, s scan.Scanner, l classify.Lookups) Page {
	var out Page
	for _, m := range items {
		verdict := classify.Classify(p, m.From.Email, l)
		masked := redact.MaskSubject(s, m.Subject)
		msg := Message{
			ID:             m.ID,
			ThreadID:       m.ThreadID,
			From:           m.From,
			Domain:         domain(m.From.Email),
			Subject:        masked.Subject(),
			Date:           m.Date,
			Labels:         slices.Clone(m.Labels),
			Flags:          m.Flags,
			HasAttachments: m.HasAttachments,
			ListID:         m.ListID,
			SizeBytes:      m.SizeBytes,
			AuthResults:    m.AuthResults,
			Stamp:          StampOf(s),
			Class:          Normal,
			ClassRule:      verdict.Rule(),
			Unclassified:   verdict.Reason() == classify.Unclassifiable,
		}
		if verdict.Class().Restricted() {
			msg.Class = Restricted
		}
		for _, e := range masked.Events() {
			msg.Masks = append(msg.Masks, Mask{Rule: e.Rule(), Tier: e.Tier()})
		}
		msg.SubjectMasked = len(msg.Masks) > 0
		out.Messages = append(out.Messages, msg)
		out.Domains = append(out.Domains, msg.Domain)
	}
	slices.Sort(out.Domains)
	out.Domains = slices.Compact(out.Domains)
	return out
}

// domain returns the stored form of the part of address after its last @, or empty when there is
// none. A caller holding a bare domain passes it to StoredDomain instead, since through domain it would
// become empty and match nothing.
func domain(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return ""
	}
	return StoredDomain(address[at+1:])
}

// StoredDomain returns a sender domain in the one form the index stores it in, which every workload
// writes and every statement that matches a sender domain is given (ADR-0016). It applies Go's simple
// lowercase mapping, strings.ToLower, after taking U+0130 (İ) to i followed by U+0307, as the one
// unconditional entry of Unicode's SpecialCasing.txt whose lowercase differs from the simple
// mapping's plain i does. It applies none of that file's conditional entries, such as the final
// sigma, because the sender classifier's UTS #46 mapping maps each character the same wherever it
// stands and takes no language, and maps U+0130 the same way. So a stored domain classifies as the
// address it came from, which classifying senders by their stored domain relies on (ADR-0108). With
// the simple mapping alone, a rule listing a domain written with U+0130 would restrict the address
// but not the stored domain.
func StoredDomain(domain string) string {
	return strings.ToLower(strings.ReplaceAll(domain, "\u0130", "i\u0307"))
}

// StoredFlags returns a message's flags in the form the index stores them, an object keyed read and
// starred, which every workload that writes a message encodes the same way so a stored message's
// flags compare equal whoever wrote them (ADR-0016).
func StoredFlags(f mail.Flags) map[string]bool {
	return map[string]bool{"read": f.Read, "starred": f.Starred}
}

// StoredAuthResults returns a message's sender authentication results in the form the index stores
// them, an object keyed spf, dkim and dmarc, or nil for a message that carries none, which the index
// stores as null (ADR-0016).
func StoredAuthResults(a mail.AuthResults) map[string]string {
	if a == (mail.AuthResults{}) {
		return nil
	}
	return map[string]string{"spf": a.SPF, "dkim": a.DKIM, "dmarc": a.DMARC}
}
