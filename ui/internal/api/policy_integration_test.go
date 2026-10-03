//go:build integration

package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// seedPolicy writes the policy the policy tests start from, as the superuser. The base policy holds
// base.bank, restricting bank.example, and personal holds operator.lender.example, restricting
// lender.example, and other holds a rule of its own. Each rule has the history row its add wrote.
func (r *rig) seedPolicy() {
	r.t.Helper()
	r.exec(`INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by, created_at) VALUES
		(NULL, 'base.bank', 'restricted', '{bank.example}', 'operator', 'operator', $1),
		($2, 'operator.lender.example', 'restricted', '{lender.example}', 'operator', 'operator', $1),
		($3, 'other.only', 'restricted', '{other.example}', 'operator', 'operator', $1)`, now.Add(-72*3600e9), personal, other)
	r.exec(`INSERT INTO policy_changes (account_id, ts, actor, action, rule_id, suffixes_after) VALUES
		(NULL, $1, 'operator', 'added', 'base.bank', '{bank.example}'),
		($2, $1, 'operator', 'added', 'operator.lender.example', '{lender.example}'),
		($3, $1, 'operator', 'added', 'other.only', '{other.example}')`, now.Add(-72*3600e9), personal, other)
}

// policyRow is a rules dataset row as a test reads it.
type policyRow struct {
	Scope      string   `json:"scope"`
	RuleID     string   `json:"rule_id"`
	Suffixes   []string `json:"suffixes"`
	Senders    int64    `json:"senders"`
	Restricted int64    `json:"restricted"`
	Messages   int64    `json:"messages"`
}

type rowsPage[T any] struct {
	Rows  []T `json:"rows"`
	Total struct {
		Count int64 `json:"count"`
	} `json:"total"`
}

type historyRow struct {
	Account        *string  `json:"account"`
	Actor          string   `json:"actor"`
	Action         string   `json:"action"`
	RuleID         string   `json:"rule_id"`
	SuffixesBefore []string `json:"suffixes_before"`
	SuffixesAfter  []string `json:"suffixes_after"`
}

// changes reads every history row as the superuser, oldest first, as scope action rule before>after.
func (r *rig) changes() []string {
	r.t.Helper()
	rows, err := r.db.Query(r.t.Context(), `SELECT coalesce(account_id, 'base') || ' ' || action || ' ' || rule_id || ' ' ||
		array_to_string(suffixes_before, ',') || '>' || array_to_string(suffixes_after, ',') || ' by ' || actor FROM policy_changes ORDER BY id`)
	if err != nil {
		r.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			r.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// rulesOf reads every stored rule as the superuser, as scope id suffixes.
func (r *rig) rulesOf() []string {
	r.t.Helper()
	rows, err := r.db.Query(r.t.Context(), `SELECT coalesce(account_id, 'base') || ' ' || rule_id || ' ' || array_to_string(domain_suffix, ',')
		FROM policy_rules ORDER BY account_id NULLS FIRST, rule_id`)
	if err != nil {
		r.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			r.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// The rules dataset lists the base rules and the account's own, each with what it matches among the
// account's senders, and its row detail is one rule of one scope. Another account's rule is neither
// listed nor read. The history lists the account's changes and the base policy's, and the senders
// dataset answers the sender picker's search with the rule restricting each sender (docs/UI.md
// sections 8.7 and 17.1).
func TestThePolicyDatasetsAnswer(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()

	page := decode[rowsPage[policyRow]](t, b.get("/api/personal/lens?dataset=rules&level=3"))
	want := []policyRow{
		{Scope: "base", RuleID: "base.bank", Suffixes: []string{"bank.example"}, Senders: 1, Restricted: 1, Messages: 1},
		{Scope: "account", RuleID: "operator.lender.example", Suffixes: []string{"lender.example"}, Senders: 1, Restricted: 0, Messages: 4},
	}
	if diff := cmp.Diff(want, page.Rows, compare.Options); diff != "" {
		t.Errorf("the rules (-want +got):\n%s", diff)
	}
	searched := decode[rowsPage[policyRow]](t, b.get("/api/personal/lens?dataset=rules&level=3&search=LENDER"))
	if len(searched.Rows) != 1 || searched.Rows[0].RuleID != "operator.lender.example" || searched.Total.Count != 1 {
		t.Errorf("the search for LENDER found %+v", searched)
	}

	detail := decode[struct {
		RuleID   string                            `json:"rule_id"`
		Scope    string                            `json:"scope"`
		Released struct{ Senders, Messages int64 } `json:"released"`
		History  []historyRow                      `json:"history"`
		Accounts []string                          `json:"accounts"`
	}](t, b.get("/api/personal/rules/base:base.bank"))
	if detail.Scope != "base" || detail.Released.Senders != 1 || len(detail.History) != 1 || !slices.Equal(detail.Accounts, []string{other, personal}) {
		t.Errorf("the base rule's detail is %+v", detail)
	}
	refused(t, b.get("/api/personal/rules/account:base.bank"), http.StatusNotFound, "unknown_row")
	refused(t, b.get("/api/personal/rules/account:other.only"), http.StatusNotFound, "unknown_row")

	history := decode[rowsPage[historyRow]](t, b.get("/api/personal/lens?dataset=policy_changes&level=3&range=all"))
	var named []string
	for _, h := range history.Rows {
		named = append(named, h.RuleID)
	}
	slices.Sort(named)
	if !slices.Equal(named, []string{"base.bank", "operator.lender.example"}) {
		t.Errorf("personal's history names %v, want its own change and the base policy's", named)
	}
	base := decode[rowsPage[historyRow]](t, b.get("/api/personal/lens?dataset=policy_changes&level=3&range=all&scope=none"))
	if len(base.Rows) != 1 || base.Rows[0].Account != nil {
		t.Errorf("the base scope's history is %+v", base.Rows)
	}

	senders := decode[rowsPage[struct {
		Domain       string  `json:"domain"`
		RestrictedBy *string `json:"restricted_by"`
	}]](t, b.get("/api/personal/lens?dataset=senders&level=3&search=example"))
	byDomain := map[string]string{}
	for _, s := range senders.Rows {
		byDomain[s.Domain] = ""
		if s.RestrictedBy != nil {
			byDomain[s.Domain] = *s.RestrictedBy
		}
	}
	if diff := cmp.Diff(map[string]string{"bank.example": "base.bank", "lender.example": "operator.lender.example"}, byDomain, compare.Options); diff != "" {
		t.Errorf("the senders and their rules (-want +got):\n%s", diff)
	}
}

// Every policy write that succeeds leaves exactly one history row naming its identity, action, rule
// and suffixes before and after, and one failed between its write to the rules and its history row
// leaves neither (ADR-0102, VERIFICATIONS, the policy history row).
func TestAPolicyWriteLeavesItsHistoryRowOrNothing(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	before := r.changes()

	failing := func(at string) error {
		if at == "between a rule's write and its history row" {
			return errors.New("an injected fault")
		}
		return nil
	}
	api.SetFault(r.s, failing)
	stored := r.stored()
	for name, res := range map[string]response{
		"add":      b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "operator.shop.example", "suffixes": []string{"shop.example"}}),
		"edit":     b.post("/api/personal/policy/rules/operator.lender.example", map[string]any{"scope": "account", "suffixes_before": []string{"lender.example"}, "suffixes": []string{"lender.example", "loans.example"}, "confirmation": nil}),
		"lift":     b.post("/api/personal/policy/rules/operator.lender.example/lift", map[string]any{"scope": "account", "suffixes_before": []string{"lender.example"}, "confirmation": nil}),
		"base add": b.post("/api/setup/policy/rules", map[string]any{"rule_id": "base.shop", "suffixes": []string{"shop.example"}}),
	} {
		if res.status != http.StatusServiceUnavailable {
			t.Errorf("%s failed between its writes answered %d, want the database's 503:\n%s", name, res.status, res.body)
		}
	}
	if got := r.stored(); got != stored {
		t.Fatalf("a write failed between its rule and its history row left:\nbefore %s\nafter  %s", stored, got)
	}
	api.SetFault(r.s, nil)

	decode[map[string]any](t, b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "operator.shop.example", "suffixes": []string{"shop.example"}}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules/operator.shop.example", map[string]any{"scope": "account", "suffixes_before": []string{"shop.example"}, "suffixes": []string{"shop.example", "store.example"}, "confirmation": nil}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules/operator.shop.example/lift", map[string]any{"scope": "account", "suffixes_before": []string{"shop.example", "store.example"}, "confirmation": nil}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules", map[string]any{"scope": "base", "rule_id": "base.shop", "suffixes": []string{"shop.example"}}))
	want := append(slices.Clone(before),
		"personal added operator.shop.example >shop.example by operator",
		"personal edited operator.shop.example shop.example>shop.example,store.example by operator",
		"personal lifted operator.shop.example shop.example,store.example> by operator",
		"base added base.shop >shop.example by operator",
	)
	if diff := cmp.Diff(want, r.changes(), compare.Options); diff != "" {
		t.Errorf("the history (-want +got):\n%s", diff)
	}
}

// A write the policy snapshot's validation would refuse, and one whose identifier a screen's address
// could not reach, is refused before anything is written, through an account's request in either scope
// and through the base policy's (ADR-0041, VERIFICATIONS, the snapshot validation row).
func TestAWriteTheValidationRefusesIsRefusedBeforeItIsWritten(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	type attempt struct {
		path string
		body map[string]any
	}
	add := func(scope, id string, suffixes ...string) attempt {
		if scope == "setup" {
			return attempt{"/api/setup/policy/rules", map[string]any{"rule_id": id, "suffixes": suffixes}}
		}
		return attempt{"/api/personal/policy/rules", map[string]any{"scope": scope, "rule_id": id, "suffixes": suffixes}}
	}
	for _, scope := range []string{"account", "base", "setup"} {
		for name, a := range map[string]attempt{
			"an empty identifier":        add(scope, "", "shop.example"),
			"an identifier with a space": add(scope, " shop", "shop.example"),
			"no suffix":                  add(scope, "operator.none"),
			"a suffix not a domain name": add(scope, "operator.bad", "*.shop.example"),
			"the route word new":         add(scope, "new", "shop.example"),
			"the route word history":     add(scope, "history", "shop.example"),
			"a dot":                      add(scope, ".", "shop.example"),
			"two dots":                   add(scope, "..", "shop.example"),
			"a slash":                    add(scope, "a/b", "shop.example"),
		} {
			res := b.post(a.path, a.body)
			if res.status != http.StatusBadRequest || !strings.Contains(string(res.body), `"rule_refused"`) {
				t.Errorf("%s in scope %s answered %d:\n%s", name, scope, res.status, res.body)
			}
		}
	}
	refused(t, b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "pick", "suffixes": []string{"shop.example"}}), http.StatusBadRequest, "rule_refused")
	refused(t, b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "operator.lender.example", "suffixes": []string{"shop.example"}}), http.StatusConflict, "identifier_taken")
	refused(t, b.post("/api/setup/policy/rules", map[string]any{"rule_id": "base.bank", "suffixes": []string{"shop.example"}}), http.StatusConflict, "identifier_taken")
	refused(t, b.post("/api/personal/policy/rules/operator.lender.example", map[string]any{"scope": "account", "suffixes_before": []string{"lender.example"}, "suffixes": []string{}, "confirmation": nil}), http.StatusBadRequest, "rule_refused")
	if got := r.stored(); got != stored {
		t.Errorf("a refused write wrote:\nbefore %s\nafter  %s", stored, got)
	}
	// The base policy's route words are not an account's, so pick names a base rule.
	decode[map[string]any](t, b.post("/api/setup/policy/rules", map[string]any{"rule_id": "pick", "suffixes": []string{"shop.example"}}))
}

// A base rule's lift, and an edit removing one of its suffixes, need the rule's identifier typed, the
// server's check and not the screen's alone, through an account's request and through the base
// policy's (docs/UI.md section 17.4, VERIFICATIONS, the typed confirmation row).
func TestABaseLiftNeedsTheIdentifierTyped(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	for _, confirmation := range []any{nil, "base", "lift 1"} {
		refused(t, b.post("/api/personal/policy/rules/base.bank/lift", map[string]any{"scope": "base", "suffixes_before": []string{"bank.example"}, "confirmation": confirmation}), http.StatusBadRequest, "confirmation_required")
		refused(t, b.post("/api/setup/policy/rules/base.bank/lift", map[string]any{"suffixes_before": []string{"bank.example"}, "confirmation": confirmation}), http.StatusBadRequest, "confirmation_required")
		refused(t, b.post("/api/personal/policy/rules/base.bank", map[string]any{"scope": "base", "suffixes_before": []string{"bank.example"}, "suffixes": []string{"bank.test"}, "confirmation": confirmation}), http.StatusBadRequest, "confirmation_required")
		refused(t, b.post("/api/setup/policy/rules/base.bank", map[string]any{"suffixes_before": []string{"bank.example"}, "suffixes": []string{"bank.test"}, "confirmation": confirmation}), http.StatusBadRequest, "confirmation_required")
	}
	if got := r.stored(); got != stored {
		t.Fatalf("an unconfirmed base lift wrote:\nbefore %s\nafter  %s", stored, got)
	}
	// An edit that only adds a suffix to a base rule lifts nothing and needs no confirmation.
	decode[map[string]any](t, b.post("/api/setup/policy/rules/base.bank", map[string]any{"suffixes_before": []string{"bank.example"}, "suffixes": []string{"bank.example", "bank.test"}, "confirmation": nil}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules/base.bank/lift", map[string]any{"scope": "base", "suffixes_before": []string{"bank.example", "bank.test"}, "confirmation": " base.bank "}))
	if slices.ContainsFunc(r.rulesOf(), func(s string) bool { return strings.HasPrefix(s, "base base.bank ") }) {
		t.Errorf("the confirmed lift left the base rule: %v", r.rulesOf())
	}
}

// Under one account, another account's rule is unknown to an edit and a lift, never a stale conflict,
// and nothing is written. A stale edit of the account's own rule is a conflict (ADR-0084, VERIFICATIONS,
// the row on a write row-level security empties).
func TestAnotherAccountsRuleIsUnknown(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	refused(t, b.post("/api/personal/policy/rules/other.only", map[string]any{"scope": "account", "suffixes_before": []string{"other.example"}, "suffixes": []string{"other.example", "x.example"}, "confirmation": nil}), http.StatusNotFound, "unknown_rule")
	refused(t, b.post("/api/personal/policy/rules/other.only/lift", map[string]any{"scope": "account", "suffixes_before": []string{"other.example"}, "confirmation": nil}), http.StatusNotFound, "unknown_rule")
	refused(t, b.post("/api/personal/policy/rules/operator.lender.example", map[string]any{"scope": "account", "suffixes_before": []string{"loans.example"}, "suffixes": []string{"lender.example", "x.example"}, "confirmation": nil}), http.StatusConflict, "stale_rule")
	refused(t, b.post("/api/personal/policy/rules/operator.lender.example/lift", map[string]any{"scope": "account", "suffixes_before": []string{"loans.example"}, "confirmation": nil}), http.StatusConflict, "stale_rule")
	if got := r.stored(); got != stored {
		t.Errorf("a refused write wrote:\nbefore %s\nafter  %s", stored, got)
	}
}

// An identifier is unique within its scope. One lifted earlier comes back in its own scope with its
// history reading added, lifted, added, and an account adds a rule of an identifier the base policy
// holds beside it, each rule's detail holding only its own scope's history (ADR-0110, VERIFICATIONS,
// the identifier row).
func TestAnIdentifierComesBackInItsScopeAndBesideTheOther(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	decode[map[string]any](t, b.post("/api/personal/policy/rules/operator.lender.example/lift", map[string]any{"scope": "account", "suffixes_before": []string{"lender.example"}, "confirmation": nil}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "operator.lender.example", "suffixes": []string{"lender.example"}}))
	decode[map[string]any](t, b.post("/api/personal/policy/rules", map[string]any{"scope": "account", "rule_id": "base.bank", "suffixes": []string{"bank.test"}}))
	type detail struct {
		Scope     string       `json:"scope"`
		Suffixes  []string     `json:"suffixes"`
		History   []historyRow `json:"history"`
		Elsewhere *struct {
			Scope string `json:"scope"`
		} `json:"elsewhere"`
	}
	lender := decode[detail](t, b.get("/api/personal/rules/account:operator.lender.example"))
	var actions []string
	for _, h := range lender.History {
		actions = append(actions, h.Action)
	}
	if !slices.Equal(actions, []string{"added", "lifted", "added"}) {
		t.Errorf("the lender rule's history reads %v, newest first, want added, lifted, added", actions)
	}
	own := decode[detail](t, b.get("/api/personal/rules/account:base.bank"))
	baseRule := decode[detail](t, b.get("/api/personal/rules/base:base.bank"))
	if own.Scope != "account" || !slices.Equal(own.Suffixes, []string{"bank.test"}) || len(own.History) != 1 || own.Elsewhere == nil || own.Elsewhere.Scope != "base" {
		t.Errorf("personal's base.bank is %+v", own)
	}
	if baseRule.Scope != "base" || !slices.Equal(baseRule.Suffixes, []string{"bank.example"}) || len(baseRule.History) != 1 || baseRule.History[0].Account != nil {
		t.Errorf("the base policy's base.bank is %+v", baseRule)
	}
}

// policyFile writes a policy file of rules, each an identifier and its suffixes.
func policyFile(rules ...[]string) string {
	if len(rules) == 0 {
		return "rules: []\n"
	}
	var b strings.Builder
	b.WriteString("rules:\n")
	for _, r := range rules {
		b.WriteString("  - id: " + r[0] + "\n    domain_suffix: [" + strings.Join(r[1:], ", ") + "]\n    class: restricted\n")
	}
	return b.String()
}

type preview struct {
	Lifts           int    `json:"lifts"`
	ComputedAgainst string `json:"computed_against"`
	Added           []struct {
		RuleID string `json:"rule_id"`
	} `json:"added"`
	Lifted []struct {
		RuleID   string                             `json:"rule_id"`
		Released *struct{ Senders, Messages int64 } `json:"released"`
	} `json:"lifted"`
	Released *struct{ Senders, Messages int64 } `json:"released"`
	Accounts []string                           `json:"accounts"`
}

// An import that lifts anything is applied only with the confirmation lift {k} for the count it lifts,
// for an account's scope and for the base scope, and one that only adds is applied with none
// (ADR-0110, VERIFICATIONS, the import confirmation row).
func TestAnImportThatLiftsNeedsItsConfirmation(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	for _, c := range []struct{ prefix, keep string }{{"/api/personal/policy/import", "operator.lender.example"}, {"/api/setup/policy/import", "base.bank"}} {
		file := policyFile([]string{"operator.shop.example", "shop.example"})
		p := decode[preview](t, b.post(c.prefix+"/preview", map[string]any{"file": file}))
		if p.Lifts != 1 || len(p.Lifted) != 1 || p.Lifted[0].RuleID != c.keep {
			t.Fatalf("%s previews %+v, want the stored rule lifted", c.prefix, p)
		}
		stored := r.stored()
		for _, confirmation := range []any{nil, "lift 2", "lift", c.keep} {
			refused(t, b.post(c.prefix, map[string]any{"file": file, "computed_against": p.ComputedAgainst, "confirmation": confirmation}), http.StatusBadRequest, "confirmation_required")
		}
		if got := r.stored(); got != stored {
			t.Fatalf("%s: an unconfirmed import wrote:\nbefore %s\nafter  %s", c.prefix, stored, got)
		}
		decode[map[string]any](t, b.post(c.prefix, map[string]any{"file": file, "computed_against": p.ComputedAgainst, "confirmation": "lift 1"}))
		adds := policyFile([]string{"operator.shop.example", "shop.example"}, []string{"operator.store.example", "store.example"})
		p = decode[preview](t, b.post(c.prefix+"/preview", map[string]any{"file": adds}))
		if p.Lifts != 0 {
			t.Fatalf("%s: a file that only adds previews %d lifts", c.prefix, p.Lifts)
		}
		decode[map[string]any](t, b.post(c.prefix, map[string]any{"file": adds, "computed_against": p.ComputedAgainst, "confirmation": nil}))
	}
	if diff := cmp.Diff([]string{
		"base operator.shop.example shop.example", "base operator.store.example store.example",
		"other other.only other.example",
		"personal operator.shop.example shop.example", "personal operator.store.example store.example",
	}, r.rulesOf(), compare.Options); diff != "" {
		t.Errorf("the rules after the imports (-want +got):\n%s", diff)
	}
}

// An import is applied whole or not at all. A fault after its first change leaves every rule and history
// row as before, and an import computed against rules another write changed since is refused with
// stale_preview (ADR-0110, VERIFICATIONS, the import transaction row).
func TestAnImportIsAppliedWholeOrNotAtAll(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	file := policyFile([]string{"operator.lender.example", "lender.example", "loans.example"}, []string{"operator.shop.example", "shop.example"}, []string{"operator.store.example", "store.example"})
	p := decode[preview](t, b.post("/api/personal/policy/import/preview", map[string]any{"file": file}))
	stored := r.stored()
	api.SetFault(r.s, func(at string) error {
		if at == "after an import's first change" {
			return errors.New("an injected fault")
		}
		return nil
	})
	if res := b.post("/api/personal/policy/import", map[string]any{"file": file, "computed_against": p.ComputedAgainst, "confirmation": nil}); res.status != http.StatusServiceUnavailable {
		t.Errorf("the import failed after its first change answered %d:\n%s", res.status, res.body)
	}
	api.SetFault(r.s, nil)
	if got := r.stored(); got != stored {
		t.Fatalf("a failed import left:\nbefore %s\nafter  %s", stored, got)
	}
	// The other write keeps every identifier and the count of rules, and changes one rule's suffixes, so
	// only a comparison of the rules themselves tells the preview is stale.
	decode[map[string]any](t, b.post("/api/personal/policy/rules/operator.lender.example", map[string]any{"scope": "account", "suffixes_before": []string{"lender.example"}, "suffixes": []string{"lender.example", "credit.example"}, "confirmation": nil}))
	stored = r.stored()
	refused(t, b.post("/api/personal/policy/import", map[string]any{"file": file, "computed_against": p.ComputedAgainst, "confirmation": nil}), http.StatusConflict, "stale_preview")
	if got := r.stored(); got != stored {
		t.Errorf("a stale import wrote:\nbefore %s\nafter  %s", stored, got)
	}
}

// refusalBody is a refused file's or write's body.
type refusalBody struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
	Problems []struct {
		Kind   string `json:"kind"`
		RuleID string `json:"rule_id"`
		Line   *int   `json:"line"`
	} `json:"problems"`
}

// A file reaches the stored rules only whole. A rule failing a write's checks, two rules of one
// identifier, text not of the file's form and a file over a megabyte are refused before any preview,
// each problem named with its line. A file exported from one account imports into another and into
// the base policy (ADR-0110, VERIFICATIONS, the file row).
func TestAFileIsCheckedWholeAndImportsIntoAnyScope(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	for name, c := range map[string]struct {
		file  string
		kinds []string
		line  int
	}{
		"a rule failing a check":      {policyFile([]string{"operator.ok", "ok.example"}, []string{"operator.bad", "bad_suffix.example"}), []string{"invalid_suffix"}, 5},
		"two rules of one identifier": {policyFile([]string{"operator.twice", "a.example"}, []string{"operator.twice", "b.example"}), []string{"repeated_identifier"}, 5},
		"a key the form holds no":     {"rules:\n  - id: x\n    domain_suffix: [a.example]\n    class: restricted\n    note: hello\n", []string{"not_the_form"}, 5},
		"not YAML of the form":        {"policy: [1, 2]\n", []string{"not_the_form"}, 1},
		"over a megabyte":             {"rules: []\n" + strings.Repeat("#", 1<<20), []string{"too_large"}, 0},
	} {
		res := b.post("/api/personal/policy/import/preview", map[string]any{"file": c.file})
		if res.status != http.StatusBadRequest {
			t.Errorf("%s answered %d:\n%s", name, res.status, res.body)
			continue
		}
		var got refusalBody
		if err := json.Unmarshal(res.body, &got); err != nil {
			t.Fatal(err)
		}
		var kinds []string
		line := 0
		for _, p := range got.Problems {
			kinds = append(kinds, p.Kind)
			if p.Line != nil {
				line = *p.Line
			}
		}
		if got.Error.Code != "file_refused" || !slices.Equal(kinds, c.kinds) || line != c.line {
			t.Errorf("%s was refused as %s with %v at line %d, want file_refused with %v at line %d", name, got.Error.Code, kinds, line, c.kinds, c.line)
		}
	}
	if got := r.stored(); got != stored {
		t.Fatalf("a refused file wrote:\nbefore %s\nafter  %s", stored, got)
	}

	res := b.get("/api/personal/policy/export")
	if res.status != http.StatusOK || res.header.Get("Content-Type") != "application/yaml" || !strings.Contains(res.header.Get("Content-Disposition"), `filename="policy-personal.yaml"`) {
		t.Fatalf("the export answered %d %v:\n%s", res.status, res.header, res.body)
	}
	exported := string(res.body)
	if want := policyFile([]string{"operator.lender.example", "lender.example"}); exported != want {
		t.Errorf("the export is\n%s\nwant\n%s", exported, want)
	}
	for _, prefix := range []string{"/api/other/policy/import", "/api/setup/policy/import"} {
		p := decode[preview](t, b.post(prefix+"/preview", map[string]any{"file": exported}))
		decode[map[string]any](t, b.post(prefix, map[string]any{"file": exported, "computed_against": p.ComputedAgainst, "confirmation": "lift 1"}))
	}
	if diff := cmp.Diff([]string{
		"base operator.lender.example lender.example",
		"other operator.lender.example lender.example",
		"personal operator.lender.example lender.example",
	}, r.rulesOf(), compare.Options); diff != "" {
		t.Errorf("the rules after importing personal's file elsewhere (-want +got):\n%s", diff)
	}
}

// An import into an account's scope carries what each lift releases in the account and what the whole
// import does, and one into the base scope carries every account's identifier and no count (docs/UI.md
// section 8.7).
func TestAPreviewCarriesTheAccountsNumbers(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	account := decode[preview](t, b.post("/api/personal/policy/import/preview", map[string]any{"file": policyFile()}))
	if len(account.Lifted) != 1 || account.Lifted[0].Released == nil || account.Lifted[0].Released.Senders != 1 || account.Lifted[0].Released.Messages != 4 || account.Released == nil || account.Released.Senders != 1 {
		t.Errorf("personal's preview is %+v", account)
	}
	base := decode[preview](t, b.post("/api/setup/policy/import/preview", map[string]any{"file": policyFile()}))
	if len(base.Lifted) != 1 || base.Lifted[0].Released != nil || base.Released != nil || !slices.Equal(base.Accounts, []string{other, personal}) {
		t.Errorf("the base policy's preview is %+v", base)
	}
}

// With an identity header declared, a policy write without it is refused before anything is written,
// and one with it records the header's value (ADR-0084, VERIFICATIONS, the request token and identity
// row).
func TestAPolicyWriteRecordsTheDeclaredIdentity(t *testing.T) {
	r := newRig(t)
	r.identity = api.Identity{Header: "X-Forwarded-User"}
	r.s = r.server(tokenKey)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	add := map[string]any{"scope": "account", "rule_id": "operator.shop.example", "suffixes": []string{"shop.example"}}
	refused(t, b.post("/api/personal/policy/rules", add), http.StatusForbidden, "identity_missing")
	refused(t, b.post("/api/setup/policy/rules", map[string]any{"rule_id": "base.shop", "suffixes": []string{"shop.example"}}), http.StatusForbidden, "identity_missing")
	if got := r.stored(); got != stored {
		t.Fatalf("a write without its identity wrote:\nbefore %s\nafter  %s", stored, got)
	}
	b.header = map[string]string{"X-Forwarded-User": "jo@example.org"}
	decode[map[string]any](t, b.post("/api/personal/policy/rules", add))
	changes := r.changes()
	if last := changes[len(changes)-1]; !strings.HasSuffix(last, " by jo@example.org") {
		t.Errorf("the write recorded %q", last)
	}
}

// The policy routes are scoped like every other, so all and an unknown account are refused before
// anything is read or written (ADR-0056, VERIFICATIONS, the per-account row).
func TestThePolicyRoutesAreScoped(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	stored := r.stored()
	for _, account := range []string{"all", "nobody"} {
		code := map[string]string{"all": "all_accounts", "nobody": "unknown_account"}[account]
		prefix := "/api/" + account
		refused(t, b.get(prefix+"/lens?dataset=rules"), http.StatusBadRequest, code)
		refused(t, b.get(prefix+"/lens?dataset=policy_changes"), http.StatusBadRequest, code)
		refused(t, b.get(prefix+"/lens?dataset=senders"), http.StatusBadRequest, code)
		refused(t, b.get(prefix+"/rules/base:base.bank"), http.StatusBadRequest, code)
		refused(t, b.get(prefix+"/policy/match?suffix=bank.example"), http.StatusBadRequest, code)
		refused(t, b.get(prefix+"/policy/export"), http.StatusBadRequest, code)
		refused(t, b.post(prefix+"/policy/rules", map[string]any{"scope": "account", "rule_id": "x", "suffixes": []string{"x.example"}}), http.StatusBadRequest, code)
		refused(t, b.post(prefix+"/policy/rules/x", map[string]any{"scope": "account", "suffixes_before": []string{}, "suffixes": []string{"x.example"}, "confirmation": nil}), http.StatusBadRequest, code)
		refused(t, b.post(prefix+"/policy/rules/x/lift", map[string]any{"scope": "account", "suffixes_before": []string{}, "confirmation": nil}), http.StatusBadRequest, code)
		refused(t, b.post(prefix+"/policy/import/preview", map[string]any{"file": policyFile()}), http.StatusBadRequest, code)
		refused(t, b.post(prefix+"/policy/import", map[string]any{"file": policyFile(), "computed_against": "[]", "confirmation": nil}), http.StatusBadRequest, code)
	}
	if got := r.stored(); got != stored {
		t.Errorf("a refused request stored:\n%s", got)
	}
}

// The base policy's installation reads hold the base rules, their history and every account's
// identifier, and no account's state, counts or rows (ADR-0056, VERIFICATIONS, the installation row).
func TestTheBasePolicyReadsNoAccountsState(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	var answers []string
	for _, path := range []string{"/api/setup/policy", "/api/setup/policy/history?range=all", "/api/setup/policy/export", "/setup/policy", "/api/setup"} {
		res := b.get(path)
		if res.status != http.StatusOK {
			t.Fatalf("%s answered %d:\n%s", path, res.status, res.body)
		}
		answers = append(answers, string(res.body))
	}
	for _, answer := range answers {
		for _, absent := range []string{"operator.lender.example", "other.only", "lender.example", `"senders"`, `"messages"`} {
			if strings.Contains(answer, absent) {
				t.Errorf("a base policy answer holds %q:\n%s", absent, answer)
			}
		}
	}
	got := decode[struct {
		Rules []struct {
			RuleID string `json:"rule_id"`
		} `json:"rules"`
		Total    int      `json:"total"`
		Accounts []string `json:"accounts"`
	}](t, b.get("/api/setup/policy?"+url.Values{"search": {"BANK"}}.Encode()))
	if len(got.Rules) != 1 || got.Rules[0].RuleID != "base.bank" || got.Total != 1 || !slices.Equal(got.Accounts, []string{other, personal}) {
		t.Errorf("the base policy reads %+v", got)
	}
	history := decode[struct {
		Rows []historyRow `json:"rows"`
	}](t, b.get("/api/setup/policy/history?range=all&rule=base.bank"))
	if len(history.Rows) != 1 || history.Rows[0].RuleID != "base.bank" {
		t.Errorf("the base rule's history is %+v", history.Rows)
	}
	if n := decode[struct {
		BaseRules int `json:"base_rules"`
	}](t, b.get("/api/setup")).BaseRules; n != 1 {
		t.Errorf("the installation counts %d base rules, want 1", n)
	}
}

// The add panel's read says which suffixes the policy's checks take, what each matches in the account,
// whether it is a public suffix, and which rules already match it (docs/UI.md section 8.7).
func TestTheAddPanelReadsEachSuffixsMatch(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	b := r.browser()
	got := decode[struct {
		SendersTotal int64 `json:"senders_total"`
		Suffixes     []struct {
			Suffix       string `json:"suffix"`
			Valid        bool   `json:"valid"`
			PublicSuffix bool   `json:"public_suffix"`
			Senders      int64  `json:"senders"`
			Messages     int64  `json:"messages"`
			Rules        []struct {
				Scope  string `json:"scope"`
				RuleID string `json:"rule_id"`
			} `json:"rules"`
		} `json:"suffixes"`
	}](t, b.get("/api/personal/policy/match?"+url.Values{"suffix": {"mail.bank.example", "example", "*.x"}}.Encode()))
	if got.SendersTotal != 2 || len(got.Suffixes) != 3 {
		t.Fatalf("the match reads %+v", got)
	}
	bank, public, bad := got.Suffixes[0], got.Suffixes[1], got.Suffixes[2]
	if !bank.Valid || bank.PublicSuffix || len(bank.Rules) != 1 || bank.Rules[0].RuleID != "base.bank" {
		t.Errorf("mail.bank.example reads %+v", bank)
	}
	if !public.Valid || !public.PublicSuffix || public.Senders != 2 || public.Messages != 5 {
		t.Errorf("example reads %+v", public)
	}
	if bad.Valid {
		t.Errorf("*.x reads %+v, want invalid", bad)
	}
}
