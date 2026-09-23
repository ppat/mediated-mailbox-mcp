// Package classify is the sender classifier. It assigns a sender class from the policy the caller
// passes in and returns the classification as a value (ADR-0004, ADR-0040).
//
// The policy list is the only authority. A sender is restricted when its domain is a domain suffix
// of a rule in the account's policy, meaning the suffix itself or any subdomain of it, and normal
// otherwise. The classifier takes no authentication result and no display name, so neither can
// change a class. Display-name matching is one of the heuristics that only propose candidates.
//
// Both sides are normalized before matching, lowercased and decoded from punycode, so a rule and a
// sender written in different forms of one name match, and every sender domain must have a
// registrable domain under the public-suffix list. The packages that decode punycode and hold the
// list are not pure, so they stay out of the pure core, and the caller passes their functions in as
// Lookups.
//
// Fail closed. A policy that never loaded restricts every sender, and an address whose domain cannot
// be read or has no registrable domain is classified restricted, never normal.
package classify

import (
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// Lookups are the functions the classifier normalizes domains with, passed in by the caller. Each
// returns an error for a name it cannot handle, and the classifier then classifies the sender
// restricted.
type Lookups struct {
	// ToUnicode returns the Unicode form of a domain name, such as
	// golang.org/x/net/idna.Lookup.ToUnicode. It must apply the UTS #46 mapping for lookup, as that
	// function does, so a name written in upper case, in full-width characters or in upper-case
	// punycode maps to the form a rule is compared in. Plain punycode decoding does not, and a
	// listed sender written that way would classify normal.
	ToUnicode func(domain string) (string, error)
	// ToASCII returns the ASCII form of a domain name, such as golang.org/x/net/idna.Lookup.ToASCII.
	ToASCII func(domain string) (string, error)
	// Registrable returns the registrable domain of a domain name in ASCII form through the
	// public-suffix list, such as golang.org/x/net/publicsuffix.EffectiveTLDPlusOne. It must refuse
	// a name with an empty label, as that function does, because the classifier relies on it to.
	Registrable func(domain string) (string, error)
}

// Reason says why a sender got its class. The zero value is unclassifiable.
type Reason uint8

const (
	// Unclassifiable is an address whose domain could not be read or has no registrable domain.
	Unclassifiable Reason = iota
	// NoPolicy is a classification made while no valid policy has loaded.
	NoPolicy
	// Listed is a domain matching a rule of the policy.
	Listed
	// Unlisted is a domain matching no rule.
	Unlisted
)

// String names the reason.
func (r Reason) String() string {
	switch r {
	case NoPolicy:
		return "no policy"
	case Listed:
		return "listed"
	case Unlisted:
		return "unlisted"
	case Unclassifiable:
		return "unclassifiable"
	default:
		return "unclassifiable"
	}
}

// Verdict is one sender's classification. The zero value is a restricted, unclassifiable sender.
// A decision reads Class. Reason and Rule explain the class, and a sender is restricted for more
// reasons than a listed rule.
type Verdict struct {
	class  sensitivity.SenderClass
	reason Reason
	rule   string
}

// Class returns the sender class.
func (v Verdict) Class() sensitivity.SenderClass { return v.class }

// Reason returns why the sender got its class.
func (v Verdict) Reason() Reason { return v.reason }

// Rule returns the identifier of the rule that matched, for a listed sender.
func (v Verdict) Rule() string { return v.rule }

// Classify returns the class of the sender of address under the account policy p.
func Classify(p policy.Composed, address string, l Lookups) Verdict {
	if p.RestrictsAll() {
		return Verdict{class: sensitivity.RestrictedSender(), reason: NoPolicy}
	}
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return Verdict{}
	}
	domain, ok := normalize(address[at+1:], l)
	if !ok {
		return Verdict{}
	}
	ascii, err := l.ToASCII(domain)
	if err != nil {
		return Verdict{}
	}
	if _, err := l.Registrable(ascii); err != nil {
		return Verdict{}
	}
	for _, r := range p.Rules() {
		for _, suffix := range r.DomainSuffixes() {
			s, ok := normalize(suffix, l)
			if !ok {
				s = strings.ToLower(suffix)
			}
			if domain == s || strings.HasSuffix(domain, "."+s) {
				return Verdict{class: sensitivity.RestrictedSender(), reason: Listed, rule: r.ID()}
			}
		}
	}
	return Verdict{class: sensitivity.NormalSender(), reason: Unlisted}
}

// normalize returns the Unicode form of a domain name, lowercased, without a trailing dot, or false
// when it is not a domain name.
func normalize(domain string, l Lookups) (string, bool) {
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return "", false
	}
	u, err := l.ToUnicode(domain)
	if err != nil {
		return "", false
	}
	return strings.ToLower(u), true
}
