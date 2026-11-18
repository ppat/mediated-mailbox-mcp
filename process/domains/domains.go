// Package domains holds the domain lookups every program classifies senders with, the IDNA
// conversions and the public suffix list the sender classifier normalizes domains through
// (ADR-0004). The classifier is a pure core and cannot import the packages that hold them, so it
// takes them from its caller as classify.Lookups (ADR-0040), and this package is the one place they
// are chosen. The policies a Session holds are prepared with them, and so is every policy the UI
// classifies under, so a domain reads as one name to every program that classifies it (ADR-0136).
package domains

import (
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
)

// Lookups returns the domain lookups every program classifies senders with, the UTS #46 mapping for lookup
// in each direction and the public suffix list's registrable domain.
func Lookups() classify.Lookups {
	return classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
}
