package api_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
)

// identifierCase is one case of the shared table of account identifier cases, and the verdict each
// side must give it (ADR-0128, VERIFICATIONS, the identifier agreement row). Server is whether account setup's check accepts
// Identifier, and Database whether the database's check on accounts.account_id stores it. The two
// agree on every case, since an account's identifier follows the word accounts in every path the UI
// serves and no word of the UI's is refused (ADR-0137). Proposal
// is what the connect page proposes for a mailbox whose local part is Identifier while the accounts
// in Taken exist, which every side must accept.
type identifierCase struct {
	Identifier string   `json:"identifier"`
	Server     bool     `json:"server"`
	Database   bool     `json:"database"`
	Taken      []string `json:"taken"`
	Proposal   string   `json:"proposal"`
}

// identifierTable is the table as the browser's test reads it.
type identifierTable struct {
	Cases []identifierCase `json:"cases"`
}

// The table's cases. They hold the grammar's edges, every kind of character a mailbox's local part
// may carry, the newlines and non-ASCII letters a regular expression dialect or a case folding could
// read differently, the product's and the UI's own path words, which are identifiers like any other,
// and the length and suffix edges of the proposal. No case
// is an account the integration tests seed, so the database's verdict is its check's alone.
var identifierCases = func() []identifierCase {
	a := func(n int) string { return strings.Repeat("a", n) }
	accepted := func(id string) identifierCase {
		return identifierCase{Identifier: id, Server: true, Database: true, Taken: []string{}, Proposal: id}
	}
	refused := func(id, proposal string) identifierCase {
		return identifierCase{Identifier: id, Taken: []string{}, Proposal: proposal}
	}
	taken := func(c identifierCase, proposal string, held ...string) identifierCase {
		c.Taken, c.Proposal = held, proposal
		return c
	}
	return []identifierCase{
		accepted("jo"),
		accepted("a"),
		accepted("7"),
		accepted("2fa"),
		accepted("9lives"),
		accepted("work-2"),
		accepted("a--b"),
		accepted("xn--bcher-kva"),
		accepted("setups"),
		accepted("main"),
		accepted(a(63)),
		refused(a(64), a(63)),
		refused(a(62)+"-b", a(62)),
		refused("", "account"),
		refused(".", "account"),
		refused("..", "account"),
		refused("/", "account"),
		refused("...", "account"),
		refused("-", "account"),
		refused("-jo", "jo"),
		refused("jo-", "jo"),
		refused("Jo", "jo"),
		refused("JO.SMITH+mail", "jo-smith-mail"),
		refused("jo.smith", "jo-smith"),
		refused("jo..smith", "jo-smith"),
		refused("jo smith", "jo-smith"),
		refused("jo_smith", "jo-smith"),
		refused("a/b", "a-b"),
		refused("a@b", "a-b"),
		refused("100%", "100"),
		refused("jo?#", "jo"),
		refused("peter's gmail", "peter-s-gmail"),
		refused("jo\n", "jo"),
		refused("\njo", "jo"),
		refused("jo\r", "jo"),
		refused("jo\tsmith", "jo-smith"),
		refused(" jo", "jo"),
		refused("josé", "jos"),
		refused("émile", "mile"),
		refused("straße", "stra-e"),
		refused("İpek", "i-pek"),
		refused("Kelvin", "kelvin"),
		refused("ｊｏ", "account"),
		refused("jо", "j"),
		accepted("setup"),
		accepted("api"),
		accepted("fonts"),
		accepted("ui"),
		accepted("mcp"),
		accepted("accounts"),
		refused("Setup", "setup"),
		refused("main.js", "main-js"),
		refused("main.css", "main-css"),
		taken(accepted("jo"), "jo-3", "jo", "jo-2"),
		taken(refused("", ""), "account-2", "account"),
		taken(accepted("setup"), "setup-3", "setup", "setup-2"),
		taken(accepted(a(63)), a(61)+"-2", a(63)),
		taken(accepted(a(63)), a(60)+"-10", a(63), a(61)+"-2", a(61)+"-3", a(61)+"-4", a(61)+"-5", a(61)+"-6",
			a(61)+"-7", a(61)+"-8", a(61)+"-9"),
		taken(accepted(a(60)+"-bc"), a(60)+"-2", a(60)+"-bc"),
	}
}()

// The server's check gives every case of the table its verdict and accepts every proposal, and the
// table's verdicts are the same for the server and the database (ADR-0128, VERIFICATIONS, the
// identifier grammar row and the identifier agreement row).
func TestTheServerCheckKeepsToTheIdentifierTable(t *testing.T) {
	for _, c := range identifierCases {
		if got := setup.Identifier(c.Identifier) == setup.None; got != c.Server {
			t.Errorf("%q: the server's check accepts it %v, the table says %v", c.Identifier, got, c.Server)
		}
		if c.Server != c.Database {
			t.Errorf("%q: the table's server verdict %v is not its database verdict %v", c.Identifier, c.Server, c.Database)
		}
		if got := setup.Identifier(c.Proposal); got != setup.None || slices.Contains(c.Taken, c.Proposal) {
			t.Errorf("%q: the proposal %q is refused with %q or taken", c.Identifier, c.Proposal, got)
		}
	}
}

// The browser's copy of the table is this table, written by -update, so the browser's test of the
// proposal reads the cases the server's and the database's tests read (ADR-0128, VERIFICATIONS, the
// identifier agreement row).
func TestTheBrowsersIdentifierTableIsTheServers(t *testing.T) {
	b, err := json.MarshalIndent(identifierTable{Cases: identifierCases}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	compare.GoldenAt(t, filepath.Join("..", "..", "browser", "test", "identifiers.json"), append(b, '\n'))
}
