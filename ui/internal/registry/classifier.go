package registry

import (
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
)

// ownRules is the key the account's own rules stand under in the policy snapshot Classifier loads,
// which holds one account's policy alone.
const ownRules = "account"

// Classifier is the account's composed policy stored, the base rules and its own as one snapshot
// transaction read them, prepared for the sender classifier with l, so every class and every
// restricting rule the UI shows is the one Classify decides under those rows (ADR-0004, ADR-0136).
// The UI reads the stored rows fresh on every read and holds no policy snapshot, so it has no last
// valid policy to keep. A stored set the policy snapshot's validation refuses classifies every
// sender restricted with no rule, as a process that never loaded a valid policy does. The API
// server and the worker keep their last valid policy snapshot when a reload fails (ADR-0041), so
// what the UI shows then can differ from what they enforce. Every write the UI makes is checked
// with that validation first, so only a write outside the programs can make such a set.
func Classifier(stored []StoredRule, l classify.Lookups) classify.Policy {
	rows := make([]policy.Row, len(stored))
	for i, r := range stored {
		account := ownRules
		if r.Base {
			account = ""
		}
		rows[i] = policy.Row{Account: account, ID: r.ID, Class: r.Class, DomainSuffixes: r.Suffixes}
	}
	snapshot, err := policy.Load(rows)
	if err != nil {
		return classify.Prepare(policy.Composed{}, l)
	}
	return classify.Prepare(snapshot.For(ownRules), l)
}
