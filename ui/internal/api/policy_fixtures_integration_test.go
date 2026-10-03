//go:build integration

package api_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// The policy recordings' fixed instant, which every rule and history row a recorded write leaves is set
// to, so the recordings do not move with the database's clock.
var policyWritten = now.Add(-time.Hour)

// The policy files the import screens' recordings send. own is the account's own rules as seeded,
// lifting drops them for a new rule, and adding keeps them and adds the new rule.
var (
	ownFile     = policyFile([]string{"operator.lender.example", "lender.example"})
	liftingFile = policyFile([]string{"operator.new.example", "new.example"})
	addingFile  = policyFile([]string{"operator.lender.example", "lender.example"}, []string{"operator.new.example", "new.example"})
	baseLifting = policyFile([]string{"base.new", "new.example"})
)

// seedPolicyScreens writes what the policy screens' recordings start from, the policy tests' rules and
// history, the senders the sender picker searches, one of whose domains carries the markup marker, and
// earlier changes a history row's Restore acts on: a lifted rule of the account's, an edit that removed
// a suffix of the account's rule, and a lifted base rule.
func (r *rig) seedPolicyScreens() {
	r.t.Helper()
	r.exec("TRUNCATE policy_rules, policy_changes RESTART IDENTITY")
	r.seedPolicy()
	r.exec(`INSERT INTO policy_changes (account_id, ts, actor, action, rule_id, suffixes_before, suffixes_after) VALUES
		($1, $2, 'operator', 'lifted', 'operator.gone.example', '{gone.example}', '{}'),
		($1, $3, 'operator', 'edited', 'operator.lender.example', '{lender.example,old.lender.example}', '{lender.example}'),
		(NULL, $4, 'operator', 'lifted', 'base.old', '{old.example}', '{}')`,
		personal, now.Add(-48*time.Hour), now.Add(-24*time.Hour), now.Add(-12*time.Hour))
	r.exec(`DELETE FROM senders WHERE account_id = $1 AND domain NOT IN ('bank.example', 'lender.example')`, personal)
	r.exec(`INSERT INTO senders (account_id, domain, message_count, first_seen, last_seen, sender_class) VALUES
		($1, 'newsletter.example', 3, $2, $3, 'normal'),
		($1, 'mail.newsletter.example', 1, $2, $3, 'normal'),
		($1, $4, 2, $2, $3, 'normal')`,
		personal, now.Add(-60*24*time.Hour), now.Add(-30*time.Hour), marker.MarkupField("senderdomain"))
}

// settleWrites sets the time of every rule and history row a recorded write left to policyWritten.
func (r *rig) settleWrites() {
	r.t.Helper()
	r.exec("UPDATE policy_rules SET created_at = $1 WHERE created_at > $2", policyWritten, now)
	r.exec("UPDATE policy_changes SET ts = $1 WHERE ts > $2", policyWritten, now)
}

// TestTheRecordedPolicyFixturesMatchTheServer records the answers the policy screens of docs/UI.md
// sections 8.7 and 8.14 read and send, the account's policy, a rule, Add a rule, the sender picker, the
// history and import, and the base policy's screens, each matching its declaration in the contract
// document (ADR-0064, ADR-0065). Every write is recorded from the seeded state, which is written again
// after it, so each recording names the state it was taken in. A browser test answers a request only
// with the recording made for that method and path.
func TestTheRecordedPolicyFixturesMatchTheServer(t *testing.T) {
	r := newRig(t)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := r.browser()
	play := func(recordings ...setupRecording) {
		t.Helper()
		for _, rec := range recordings {
			recordSetup(t, doc, rec, b.do(rec.method, rec.path, rec.body))
		}
		r.settleWrites()
	}
	get := func(name, path, pattern string) setupRecording {
		return setupRecording{name, http.MethodGet, path, pattern, nil}
	}
	post := func(name, path, pattern string, body any) setupRecording {
		return setupRecording{name, http.MethodPost, path, pattern, body}
	}
	const (
		lens       = "/api/{account}/lens"
		row        = "/api/{account}/rules/{row}"
		match      = "/api/{account}/policy/match"
		baseMatch  = "/api/setup/policy/match"
		release    = "/api/{account}/policy/release"
		add        = "/api/{account}/policy/rules"
		edit       = "/api/{account}/policy/rules/{rule}"
		lift       = "/api/{account}/policy/rules/{rule}/lift"
		previewed  = "/api/{account}/policy/import/preview"
		apply      = "/api/{account}/policy/import"
		rules      = "/api/personal/lens?dataset=rules&level=3&range=all&sort=rule_id,asc&page=1"
		figures    = "/api/personal/lens?dataset=rules&level=0&range=all&sort=rule_id,asc"
		lender     = "/api/personal/rules/account%3Aoperator.lender.example"
		basePolicy = "/api/setup/policy"
	)
	accountAdd := func(scope, id string, suffixes ...string) map[string]any {
		return map[string]any{"scope": scope, "rule_id": id, "suffixes": suffixes}
	}

	// The seeded state's reads.
	r.seedPolicyScreens()
	play(
		get("rules-summary.json", figures, lens),
		get("rules-rows.json", rules, lens),
		get("rules-summary-search.json", "/api/personal/lens?dataset=rules&level=0&range=all&sort=rule_id,asc&search=bank", lens),
		get("rules-rows-search.json", "/api/personal/lens?dataset=rules&level=3&range=all&sort=rule_id,asc&page=1&search=bank", lens),
		get("rule-base-bank.json", "/api/personal/rules/base%3Abase.bank", row),
		get("rule-lender.json", lender, row),
		get("error-unknown-rule-row.json", "/api/personal/rules/account%3Abase.bank", row),
		get("senders-summary.json", "/api/personal/lens?dataset=senders&level=0&sort=message_count,desc", lens),
		get("senders-rows.json", "/api/personal/lens?dataset=senders&level=3&sort=message_count,desc&page=1", lens),
		get("senders-summary-search.json", "/api/personal/lens?dataset=senders&level=0&sort=message_count,desc&search=newsletter", lens),
		get("senders-rows-search.json", "/api/personal/lens?dataset=senders&level=3&sort=message_count,desc&page=1&search=newsletter", lens),
		get("changes-summary.json", "/api/personal/lens?dataset=policy_changes&level=0&range=30d&sort=ts,desc", lens),
		get("changes-rows.json", "/api/personal/lens?dataset=policy_changes&level=3&range=30d&sort=ts,desc&page=1", lens),
		get("match-newsletter.json", "/api/personal/policy/match?suffix=newsletter.example", match),
		get("match-newsletter-both.json", "/api/personal/policy/match?suffix=newsletter.example&suffix=mail.newsletter.example", match),
		get("match-public.json", "/api/personal/policy/match?suffix=newsletter.example&suffix=com", match),
		get("match-invalid.json", "/api/personal/policy/match?suffix=newsletter.example&suffix=bad+suffix", match),
		get("match-lender.json", "/api/personal/policy/match?suffix=lender.example", match),
		get("match-old.json", "/api/personal/policy/match?suffix=old.example", match),
		get("base-match-invalid.json", "/api/setup/policy/match?suffix=bad+suffix", baseMatch),
		get("base-match-newsletter.json", "/api/setup/policy/match?suffix=newsletter.example", baseMatch),
		get("base-match-public.json", "/api/setup/policy/match?suffix=com", baseMatch),
		get("base-match-bank.json", "/api/setup/policy/match?suffix=bank.example", baseMatch),
		get("release-bank.json", "/api/personal/policy/release?scope=base&rule=base.bank", release),
		get("base-policy.json", basePolicy, basePolicy),
		get("base-policy-search.json", basePolicy+"?search=bank", basePolicy),
		get("base-history.json", "/api/setup/policy/history?range=30d", "/api/setup/policy/history"),
		get("base-history-bank.json", "/api/setup/policy/history?range=all&rule=base.bank", "/api/setup/policy/history"),
		post("error-rule-refused.json", "/api/personal/policy/rules", add, accountAdd("account", "new", "bad suffix")),
		post("error-identifier-taken.json", "/api/personal/policy/rules", add, accountAdd("account", "operator.lender.example", "lender.example")),
		post("preview-refused.json", "/api/personal/policy/import/preview", previewed, map[string]any{"file": "rules: [not"}),
		post("preview-equal.json", "/api/personal/policy/import/preview", previewed, map[string]any{"file": ownFile}),
		post("preview-lifting.json", "/api/personal/policy/import/preview", previewed, map[string]any{"file": liftingFile}),
		post("preview-adding.json", "/api/personal/policy/import/preview", previewed, map[string]any{"file": addingFile}),
		post("error-stale-preview.json", "/api/personal/policy/import", apply,
			map[string]any{"file": liftingFile, "computed_against": "stale", "confirmation": "lift 1"}),
		post("base-preview-lifting.json", "/api/setup/policy/import/preview", "/api/setup/policy/import/preview", map[string]any{"file": baseLifting}),
	)

	// Adding a rule from the account's screen.
	play(
		post("rule-added.json", "/api/personal/policy/rules", add, accountAdd("account", "operator.newsletter.example", "newsletter.example")),
	)
	play(
		get("rule-newsletter.json", "/api/personal/rules/account%3Aoperator.newsletter.example", row),
		get("rules-rows-added.json", rules, lens),
		get("rules-summary-added.json", figures, lens),
	)

	// Adding a suffix to the account's rule, and removing it again.
	r.seedPolicyScreens()
	play(post("rule-edited.json", "/api/personal/policy/rules/operator.lender.example", edit, map[string]any{
		"scope": "account", "suffixes_before": []string{"lender.example"}, "suffixes": []string{"lender.example", "lendermail.example"}, "confirmation": nil,
	}))
	play(
		get("rule-lender-two.json", lender, row),
		get("release-lendermail.json", "/api/personal/policy/release?scope=account&rule=operator.lender.example&keep=lender.example", release),
	)
	play(post("rule-suffix-removed.json", "/api/personal/policy/rules/operator.lender.example", edit, map[string]any{
		"scope": "account", "suffixes_before": []string{"lender.example", "lendermail.example"}, "suffixes": []string{"lender.example"}, "confirmation": nil,
	}))
	play(get("rule-lender-removed.json", lender, row))

	// A rule of three suffixes, two of which cover one sender together, so removing both releases
	// more than the sum of removing each alone.
	r.seedPolicyScreens()
	decode[api.RuleAnswer](t, b.post("/api/personal/policy/rules/operator.lender.example", map[string]any{
		"scope": "account", "suffixes_before": []string{"lender.example"},
		"suffixes": []string{"lender.example", "newsletter.example", "mail.newsletter.example"}, "confirmation": nil,
	}))
	r.settleWrites()
	play(
		get("rule-lender-three.json", lender, row),
		get("release-newsletters.json", "/api/personal/policy/release?scope=account&rule=operator.lender.example&keep=lender.example", release),
	)

	// Lifting the account's rule, and putting it back.
	r.seedPolicyScreens()
	play(post("rule-lifted.json", "/api/personal/policy/rules/operator.lender.example/lift", lift, map[string]any{
		"scope": "account", "suffixes_before": []string{"lender.example"}, "confirmation": nil,
	}))
	play(get("rules-rows-lifted.json", rules, lens), get("rules-summary-lifted.json", figures, lens))
	play(post("rule-put-back.json", "/api/personal/policy/rules", add, accountAdd("account", "operator.lender.example", "lender.example")))
	play(get("rules-rows-put-back.json", rules, lens), get("rules-summary-put-back.json", figures, lens))

	// Lifting the base rule from the account's screen, the identifier typed.
	r.seedPolicyScreens()
	play(post("base-lifted-from-account.json", "/api/personal/policy/rules/base.bank/lift", lift, map[string]any{
		"scope": "base", "suffixes_before": []string{"bank.example"}, "confirmation": "base.bank",
	}))
	play(get("rules-rows-base-lifted.json", rules, lens), get("rules-summary-base-lifted.json", figures, lens))

	// Moving the account's rule to every account, the add before the lift. A second add of the same
	// identifier to the base policy is refused, as a move whose add fails is.
	r.seedPolicyScreens()
	play(post("rule-moved-add.json", "/api/personal/policy/rules", add, accountAdd("base", "operator.lender.example", "lender.example")))
	play(
		get("rule-lender-covered.json", lender, row),
		post("error-identifier-taken-base.json", "/api/personal/policy/rules", add, accountAdd("base", "operator.lender.example", "lender.example")),
	)
	play(post("rule-moved-lift.json", "/api/personal/policy/rules/operator.lender.example/lift", lift, map[string]any{
		"scope": "account", "suffixes_before": []string{"lender.example"}, "confirmation": nil,
	}))
	play(get("rule-lender-base.json", "/api/personal/rules/base%3Aoperator.lender.example", row))

	// Importing a file that lifts the account's rule, and putting the lifted rule back.
	r.seedPolicyScreens()
	against := decode[preview](t, b.post("/api/personal/policy/import/preview", map[string]any{"file": liftingFile})).ComputedAgainst
	play(post("imported.json", "/api/personal/policy/import", apply, map[string]any{"file": liftingFile, "computed_against": against, "confirmation": "lift 1"}))
	play(get("rules-rows-imported.json", rules, lens), get("rules-summary-imported.json", figures, lens))
	play(post("imported-put-back.json", "/api/personal/policy/rules", add, accountAdd("account", "operator.lender.example", "lender.example")))

	// Importing a file that only adds.
	r.seedPolicyScreens()
	against = decode[preview](t, b.post("/api/personal/policy/import/preview", map[string]any{"file": addingFile})).ComputedAgainst
	play(post("imported-adding.json", "/api/personal/policy/import", apply, map[string]any{"file": addingFile, "computed_against": against, "confirmation": nil}))

	// The base policy's own screen adds a rule, lifts one and imports a file.
	r.seedPolicyScreens()
	play(post("base-added.json", "/api/setup/policy/rules", "/api/setup/policy/rules", map[string]any{"rule_id": "base.newsletter", "suffixes": []string{"newsletter.example"}}))
	play(
		get("base-policy-added.json", basePolicy, basePolicy),
		get("base-history-newsletter.json", "/api/setup/policy/history?range=all&rule=base.newsletter", "/api/setup/policy/history"),
	)
	r.seedPolicyScreens()
	play(post("base-lifted.json", "/api/setup/policy/rules/base.bank/lift", "/api/setup/policy/rules/{rule}/lift", map[string]any{
		"suffixes_before": []string{"bank.example"}, "confirmation": "base.bank",
	}))
	play(get("base-policy-lifted.json", basePolicy, basePolicy))
	r.seedPolicyScreens()
	against = decode[preview](t, b.post("/api/setup/policy/import/preview", map[string]any{"file": baseLifting})).ComputedAgainst
	play(post("base-imported.json", "/api/setup/policy/import", "/api/setup/policy/import", map[string]any{"file": baseLifting, "computed_against": against, "confirmation": "lift 1"}))
	play(get("base-policy-imported.json", basePolicy, basePolicy))

	// An installation with base rules and no account yet.
	r.exec("TRUNCATE accounts CASCADE")
	r.exec(`INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by, created_at) VALUES
		(NULL, 'base.bank', 'restricted', '{bank.example}', 'operator', 'operator', $1)`, now.Add(-72*time.Hour))
	play(
		get("setup-base-rules.json", "/api/setup", "/api/setup"),
		get("base-policy-no-accounts.json", basePolicy, basePolicy),
	)
}
