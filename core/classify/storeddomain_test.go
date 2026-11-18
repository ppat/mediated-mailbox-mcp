package classify_test

import (
	"testing"
	"unicode/utf8"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
)

// The stored form of a domain is its lowercase by Go's simple mapping with the dotted capital I taken
// to i followed by U+0307, where the simple mapping alone gives a plain i (ADR-0016).
func TestStoredDomain(t *testing.T) {
	for in, want := range map[string]string{
		"":                         "",
		"bank.example":             "bank.example",
		"Alerts.BANK.Example":      "alerts.bank.example",
		"B\u0130NK.example":        "bi\u0307nk.example",
		"XN--BCHER-KVA.example":    "xn--bcher-kva.example",
		"B\u00dcCHER.example":      "b\u00fccher.example",
		"\uff22\uff21\uff2e\uff2b": "\uff42\uff41\uff4e\uff4b",
	} {
		if got := classify.StoredDomain(in); got != want {
			t.Errorf("StoredDomain(%+q) = %+q, want %+q", in, got, want)
		}
	}
}

// For every code point, a domain holding it and the domain's stored form are one name to the sender
// classifier, so the lowercase mapping StoredDomain applies agrees with the classifier's own for each,
// and a change to either that splits them for any code point fails here.
func TestEveryCodePointStoresAsTheClassifierReadsIt(t *testing.T) {
	checked := 0
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		for _, d := range []string{"x" + string(r) + ".example", "example." + string(r)} {
			name, ok := classify.SenderName(d, lookups)
			storedName, storedOK := classify.SenderName(classify.StoredDomain(d), lookups)
			if name != storedName || ok != storedOK {
				t.Errorf("%+q reads as %v (%v) and its stored form %+q as %v (%v)", d, name, ok, classify.StoredDomain(d), storedName, storedOK)
			}
			if ok {
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no domain was readable, so nothing was compared")
	}
}
