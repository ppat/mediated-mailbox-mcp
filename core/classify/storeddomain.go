package classify

import "strings"

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
