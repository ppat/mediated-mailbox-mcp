package provider

import (
	"github.com/ppat/mediated-mailbox-mcp/provider/internal/adapters"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"
)

// gmailName is the Gmail adapter's name, as an account names its provider and as the provider label
// of the shared provider series carries it.
const gmailName = "gmail"

// Facts are what the rest of the system needs to know about a provider without naming its adapter.
// Each adapter of a list of adapters declares its provider's, which Adapters.Facts returns.
type Facts = adapters.Facts

// FactsOf returns the facts about the provider an account names as the list of shipped adapters
// declares them, and whether a shipped adapter serves it, for code that holds no list. The list is
// the facts' one home.
func FactsOf(name string) (Facts, bool) {
	return Shipped(nil).Facts(name)
}

// gmailFacts are the facts about Gmail, whose accounts connect through an OAuth client and whose
// ceiling its adapter's rate profile declares.
func gmailFacts() Facts {
	return Facts{UsesClient: true, Ceiling: gmail.Profile{}.BudgetPerSecond()}
}
