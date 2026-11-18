package registry_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// The UI classifies under the account's composed policy as the classifier does, the base rules met
// before the account's own whatever order the statement read them in. A stored set the snapshot's
// validation refuses classifies every sender restricted with no rule, since the UI holds no last
// valid policy to keep (ADR-0004, ADR-0041, ADR-0136, VERIFICATIONS, the UI's classification under
// the policy in force).
func TestTheUIClassifiesAsTheClassifierDoes(t *testing.T) {
	stored := []registry.StoredRule{
		{ID: "a.own", Class: "restricted", Suffixes: []string{"mail.bank.example"}},
		{Base: true, ID: "b.base", Class: "restricted", Suffixes: []string{"bank.example"}},
	}
	type verdict struct {
		Restricted bool
		Rule       string
	}
	of := func(p classify.Policy, domain string) verdict {
		v := classify.Classify(p, "@"+domain)
		return verdict{v.Class().Restricted(), v.Rule()}
	}
	valid := registry.Classifier(stored, lookups)
	for domain, want := range map[string]verdict{
		"mail.bank.example": {true, "b.base"},
		"bank.example":      {true, "b.base"},
		"shop.example":      {false, ""},
	} {
		if got := of(valid, domain); got != want {
			t.Errorf("%s: %+v, want %+v", domain, got, want)
		}
	}
	invalid := registry.Classifier(append(stored, registry.StoredRule{ID: "c.own", Class: "restricted"}), lookups)
	for _, domain := range []string{"bank.example", "shop.example"} {
		if got := of(invalid, domain); got != (verdict{true, ""}) {
			t.Errorf("under a set the validation refuses, %s: %+v, want restricted with no rule", domain, got)
		}
	}
}
