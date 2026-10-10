//go:build integration

package check_test

import (
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/index"
)

// The SQLSTATE codes a check and a not-null column raise.
const (
	checkViolation   = "23514"
	notNullViolation = "23502"
)

// vocabulary is one checked column, with a statement storing a row whose value of that column is $1,
// values outside its closed set, and every value inside it.
type vocabulary struct {
	column  string
	insert  string
	refused []any
	stored  []any
}

// The closed vocabularies ADR-0016 checks, in its three tiers, each with every value of its set as
// that record writes it. The sets are written out here rather than read from any code, so a check that
// lost a value or gained one fails this test.
var vocabularies = []vocabulary{
	// Irreversible if a wrong spelling slips.
	{
		column:  "policy_changes.action",
		insert:  "INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES ('" + accountA + "', 'operator', $1, 'rule')",
		refused: []any{"removed", "Added", "deleted"},
		stored:  []any{"added", "edited", "lifted", "confirmed"},
	},
	// Asked for by a reader.
	{
		column:  "job_runs.state",
		insert:  "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) VALUES ('" + accountA + "', gen_random_uuid()::text, 'sync', 'tick', $1, now())",
		refused: []any{"pending", "Running", "cancelled"},
		stored:  []any{"running", "succeeded", "failed"},
	},
	// Cheap, next to safety.
	{
		column:  "messages.sender_class",
		insert:  "INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES ('" + accountA + "', gen_random_uuid()::text, 't', 'a@example.com', 'example.com', now(), false, $1)",
		refused: []any{"sensitive", "Restricted", ""},
		stored:  []any{"normal", "restricted"},
	},
	{
		column:  "messages.scan_state",
		insert:  "INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, scan_state) VALUES ('" + accountA + "', gen_random_uuid()::text, 't', 'a@example.com', 'example.com', now(), false, 'normal', $1)",
		refused: []any{"SKIPPED_GATE", "skipped", "clean"},
		stored:  []any{"scanned", "skipped_restricted", "skipped_gate", "pending"},
	},
	{
		column:  "messages.content_flags",
		insert:  "INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, content_flags) VALUES ('" + accountA + "', gen_random_uuid()::text, 't', 'a@example.com', 'example.com', now(), false, 'normal', $1::text[])",
		refused: []any{"{password}", "{mfa_code,MFA_CODE}", "{login_link,otp}"},
		stored:  []any{"{}", "{mfa_code}", "{login_link}", "{mfa_code,login_link}"},
	},
	{
		column:  "senders.sender_class",
		insert:  "INSERT INTO senders (account_id, domain, sender_class) VALUES ('" + accountA + "', gen_random_uuid()::text, $1)",
		refused: []any{"sensitive", "NORMAL"},
		stored:  []any{"normal", "restricted"},
	},
	{
		column:  "scan_gate_decisions.decision",
		insert:  "INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ('" + accountA + "', gen_random_uuid()::text, $1, 'restricted')",
		refused: []any{"scan", "SKIPPED", "DEFER"},
		stored:  []any{"SCAN", "SKIP"},
	},
	{
		column:  "account_state.last_auth_outcome",
		insert:  "INSERT INTO account_state (account_id, last_auth_outcome) VALUES ('" + accountA + "', $1) ON CONFLICT (account_id) DO UPDATE SET last_auth_outcome = excluded.last_auth_outcome",
		refused: []any{"ok", "Succeeded", "expired"},
		stored:  []any{"succeeded", "refused", "failed"},
	},
	{
		column:  "policy_rules.class",
		insert:  "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ('" + accountA + "', gen_random_uuid()::text, $1, '{example.com}', 'operator', 'operator')",
		refused: []any{"normal", "Restricted", "sensitive"},
		stored:  []any{"restricted"},
	},
	{
		column:  "policy_rules.source",
		insert:  "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ('" + accountA + "', gen_random_uuid()::text, 'restricted', '{example.com}', $1, 'operator')",
		refused: []any{"heuristic", "Operator", "import"},
		stored:  []any{"operator", "candidate"},
	},
	{
		column:  "policy_candidates.status",
		insert:  "INSERT INTO policy_candidates (account_id, domain, signals, score, status) VALUES ('" + accountA + "', gen_random_uuid()::text, '[]', 0.5, $1)",
		refused: []any{"accepted", "Pending", "rejected"},
		stored:  []any{"pending", "confirmed", "dismissed"},
	},
	{
		column:  "reorg_plans.status",
		insert:  "INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), '" + accountA + "', $1, '{}')",
		refused: []any{"draft", "ROLLBACK_REQUESTED", "FAILED"},
		stored:  []any{"DRAFT", "APPROVED", "APPLYING", "APPLIED", "ROLLED_BACK", "REJECTED", "APPLY_REFUSED"},
	},
	{
		column:  "rate_grants.class",
		insert:  "INSERT INTO rate_grants (account_id, class, tokens, issued_at) VALUES ('" + accountA + "', $1, 1, now())",
		refused: []any{"background", "Batch", "realtime"},
		stored:  []any{"interactive", "sync", "batch"},
	},
	{
		column:  "masking_events.field",
		insert:  "INSERT INTO masking_events (account_id, message_id, field, rule_id, tier) VALUES ('" + accountA + "', 'm-1', $1, 'rule', 1)",
		refused: []any{"body", "Subject", "snippet"},
		stored:  []any{"subject"},
	},
}

// F11's row for the checked vocabularies. Storing a value outside a checked column's closed set is
// refused by the schema, whoever writes it, and every value inside the set is stored, so the refusal
// is the set's and not the column's (ADR-0016).
func TestEachCheckedColumnRefusesAValueOutsideItsSet(t *testing.T) {
	tx := seeded(t)
	for _, v := range vocabularies {
		for _, value := range v.refused {
			if code := attempt(t, tx, v.insert, value); code != checkViolation {
				t.Errorf("storing %v in %s: %q, want %s", value, v.column, code, checkViolation)
			}
		}
		for _, value := range v.stored {
			if code := attempt(t, tx, v.insert, value); code != "" {
				t.Errorf("storing %v in %s: %q, want it stored", value, v.column, code)
			}
		}
	}
}

// F11's row for the job kind and pass of a run. A run's workload and pass form a closed set of the
// pairs the built job kinds record, so a pair no job kind records is refused, even when each word is
// one some job kind uses, and a run with no pass is refused. Each recorded pair is stored (ADR-0016).
func TestARunRecordsOneOfTheBuiltJobKindsPairs(t *testing.T) {
	tx := seeded(t)
	const insert = "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) VALUES ('" + accountA + "', gen_random_uuid()::text, $1, $2, 'running', now())"
	for _, pair := range [][2]string{{"apply", "apply"}, {"apply", "rollback"}, {"heuristics", "run"}, {"sync", "pass1"}, {"backfill", "tick"}, {"Backfill", "pass1"}} {
		if code := attempt(t, tx, insert, pair[0], pair[1]); code != checkViolation {
			t.Errorf("storing a run of %v: %q, want %s", pair, code, checkViolation)
		}
	}
	for _, pair := range [][2]string{{"backfill", "pass1"}, {"backfill", "pass2"}, {"sync", "tick"}, {"sync", "gap_recovery"}} {
		if code := attempt(t, tx, insert, pair[0], pair[1]); code != "" {
			t.Errorf("storing a run of %v: %q, want it stored", pair, code)
		}
	}
	if code := attempt(t, tx, insert, "sync", nil); code != notNullViolation {
		t.Errorf("storing a run with no pass: %q, want %s", code, notNullViolation)
	}
}

// F11's row for the sender domains. A domain holding an ASCII capital is refused in each column that
// stores a sender domain, whatever writes it, and the form the Go normalizer gives is stored, a domain
// of non-ASCII letters included, so the check refuses only what the normalizer never produces
// (ADR-0016).
func TestASenderDomainIsStoredInTheNormalizersForm(t *testing.T) {
	tx := seeded(t)
	inserts := map[string]string{
		"messages.from_domain":     "INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES ('" + accountA + "', gen_random_uuid()::text, 't', 'a@example.com', $1, now(), false, 'normal')",
		"senders.domain":           "INSERT INTO senders (account_id, domain) VALUES ('" + accountA + "', $1)",
		"policy_candidates.domain": "INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ('" + accountB + "', $1, '[]', 0.5)",
	}
	for column, insert := range inserts {
		for _, domain := range []string{"Example.com", "EXAMPLE.COM", "mail.Bank.example"} {
			if code := attempt(t, tx, insert, domain); code != checkViolation {
				t.Errorf("storing %q in %s: %q, want %s", domain, column, code, checkViolation)
			}
		}
		for _, domain := range []string{"Example.com", "ÉXAMPLE.İT", "xn--bcher-kva.example"} {
			stored := index.StoredDomain(domain)
			if code := attempt(t, tx, insert, stored); code != "" {
				t.Errorf("storing the normalizer's %q for %q in %s: %q, want it stored", stored, domain, column, code)
			}
		}
	}
}

// D1's row for the attachment media at rest. A media type or an extension outside the normalized form
// ADR-0123 states is refused by the schema, whoever writes it, so no longer or other text the sender
// wrote is stored, and every form the normalization gives is stored, an empty one included.
func TestAttachmentMediaHoldsOnlyTheNormalizedForm(t *testing.T) {
	tx := seeded(t)
	const message = "INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES ('" + accountA + "', 'm-media', 't', 'a@example.com', 'example.com', now(), true, 'normal')"
	if code := attempt(t, tx, message); code != "" {
		t.Fatalf("storing the message: %q", code)
	}
	const insert = "INSERT INTO attachment_media (account_id, message_id, media_type, extension) VALUES ('" + accountA + "', 'm-media', $1, $2)"
	long := strings.Repeat("a", 127)
	for _, pair := range [][2]string{
		{"Application/PDF", "pdf"},
		{"application/pdf; name=a.pdf", "pdf"},
		{"application/pdf", "PDF"},
		{"application", ""},
		{"application/", ""},
		{"/pdf", ""},
		{"application/-pdf", ""},
		{"text/" + long + "a", ""},
		{"application/x pdf", ""},
		{"application/p\u00e9", ""},
		{"", "statement.pdf"},
		{"", "p d f"},
		{"", "abcdefghijklmnopq"},
		{"", "r\u00e9s"},
		{"", "mmfieldmarker-statement"},
	} {
		if code := attempt(t, tx, insert, pair[0], pair[1]); code != checkViolation {
			t.Errorf("storing the media %q: %q, want %s", pair, code, checkViolation)
		}
	}
	for _, pair := range [][2]string{
		{"", ""},
		{"application/pdf", "pdf"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"},
		{"image/svg+xml", "svg"},
		{"application/x-419283", "419283"},
		{"text/" + long, ""},
		{"application/octet-stream", "abcdefghijklmnop"},
		{"message/rfc822", "eml"},
	} {
		if code := attempt(t, tx, insert, pair[0], pair[1]); code != "" {
			t.Errorf("storing the media %q: %q, want it stored", pair, code)
		}
	}
}
