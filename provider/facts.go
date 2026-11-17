package provider

import "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"

// gmailName is the Gmail adapter's name, as an account names its provider and as the provider label
// of the shared provider series carries it.
const gmailName = "gmail"

// Facts are what the rest of the system needs to know about a provider without naming its adapter.
type Facts struct {
	// UsesClient is whether the provider's accounts connect through an OAuth client of the
	// installation (ADR-0106).
	UsesClient bool
	// Ceiling is the provider's declared budget per second, in its own units, which its rate limits
	// derive from (ADR-0023, ADR-0024).
	Ceiling float64
}

// FactsOf returns the facts about the provider an account names, and whether a shipped adapter
// serves it.
func FactsOf(name string) (Facts, bool) {
	switch name {
	case gmailName:
		return Facts{UsesClient: true, Ceiling: gmail.Profile{}.BudgetPerSecond()}, true
	default:
		return Facts{}, false
	}
}
