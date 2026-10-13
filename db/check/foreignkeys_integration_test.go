//go:build integration

package check_test

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

// restrictViolation is the SQLSTATE a key declared ON DELETE RESTRICT raises.
const restrictViolation = "23001"

// countOf returns how many rows of table belong to account, read as the superuser the test connects as.
func countOf(t *testing.T, tx pgx.Tx, table, account string) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(t.Context(), "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()+" WHERE account_id = $1", account).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestEveryLogTableRefusesAnOrphan is F2's row for the foreign keys of the log tables (ADR-0016). A
// gate decision for a message the index does not hold, a run's event or failure for a run never
// recorded, and a masking event, a candidate and an audit row for an account that does not exist are
// each refused. The same rows naming a stored parent are written, so each refusal is the key's.
func TestEveryLogTableRefusesAnOrphan(t *testing.T) {
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES ('" + accountA + "', 'm-stored', 't', 'a@example.com', 'example.com', now(), false, 'normal')",
		"INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) VALUES ('" + accountA + "', 'run-stored', 'sync', 'tick', 'running', now())",
	} {
		if code := attempt(t, tx, sql); code != "" {
			t.Fatalf("%s: %s", sql, code)
		}
	}
	cases := []struct {
		name            string
		orphan, adopted string
	}{
		{
			"a gate decision",
			"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ('" + accountA + "', 'm-missing', 'SCAN', 'restricted')",
			"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ('" + accountA + "', 'm-stored', 'SCAN', 'restricted')",
		},
		{
			"a gate decision for another account's message",
			"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ('" + accountB + "', 'm-stored', 'SCAN', 'restricted')",
			"",
		},
		{
			"a run's event",
			"INSERT INTO job_run_events (account_id, run_id, kind) VALUES ('" + accountA + "', 'run-missing', 'start')",
			"INSERT INTO job_run_events (account_id, run_id, kind) VALUES ('" + accountA + "', 'run-stored', 'start')",
		},
		{
			"a run's failure",
			"INSERT INTO job_run_failures (account_id, run_id, item_kind, item_id, error_class, first_at, last_at) VALUES ('" + accountA + "', 'run-missing', 'page', '1', 'throttled', now(), now())",
			"INSERT INTO job_run_failures (account_id, run_id, item_kind, item_id, error_class, first_at, last_at) VALUES ('" + accountA + "', 'run-stored', 'page', '1', 'throttled', now(), now())",
		},
		{
			"a masking event",
			"INSERT INTO masking_events (account_id, message_id, field, rule_id, tier) VALUES ('acct-missing', 'm-1', 'subject', 'rule', 1)",
			"INSERT INTO masking_events (account_id, message_id, field, rule_id, tier) VALUES ('" + accountA + "', 'm-1', 'subject', 'rule', 1)",
		},
		{
			"a candidate",
			"INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ('acct-missing', 'example.net', '[]', 0.5)",
			"INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ('" + accountA + "', 'example.net', '[]', 0.5)",
		},
		{
			"an audit row",
			"INSERT INTO audit_log (account_id, actor, action, stage, reason, sender_class, scan_state) VALUES ('acct-missing', 'agent', 'READ_BODY', 'serve', 'released', 'normal', 'scanned')",
			"INSERT INTO audit_log (account_id, actor, action, stage, reason, sender_class, scan_state) VALUES ('" + accountA + "', 'agent', 'READ_BODY', 'serve', 'released', 'normal', 'scanned')",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code := attempt(t, tx, c.orphan); code != foreignKeyViolation {
				t.Errorf("%s: %q, want %s", c.orphan, code, foreignKeyViolation)
			}
			if c.adopted == "" {
				return
			}
			if code := attempt(t, tx, c.adopted); code != "" {
				t.Errorf("%s: %q, want it written", c.adopted, code)
			}
		})
	}
}

// TestAMessageOrRunRemovedTakesItsPartsWithIt is F2's row for the cascades (ADR-0016). A gate decision
// is the message's current state, so removing the message removes it, and a run's events and failures
// are parts of the run, so removing the run removes them. Another message's decision and another
// run's events stay, so the removal is the cascade's and not a table emptied.
func TestAMessageOrRunRemovedTakesItsPartsWithIt(t *testing.T) {
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES " +
			"('" + accountA + "', 'm-removed', 't', 'a@example.com', 'example.com', now(), false, 'normal'), " +
			"('" + accountA + "', 'm-kept', 't', 'a@example.com', 'example.com', now(), false, 'normal')",
		"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ('" + accountA + "', 'm-removed', 'SKIP', 'restricted'), ('" + accountA + "', 'm-kept', 'SCAN', 'restricted')",
		"INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) VALUES ('" + accountA + "', 'run-removed', 'sync', 'tick', 'failed', now()), ('" + accountA + "', 'run-kept', 'sync', 'tick', 'failed', now())",
		"INSERT INTO job_run_events (account_id, run_id, kind) VALUES ('" + accountA + "', 'run-removed', 'start'), ('" + accountA + "', 'run-kept', 'start')",
		"INSERT INTO job_run_failures (account_id, run_id, item_kind, item_id, error_class, first_at, last_at) VALUES " +
			"('" + accountA + "', 'run-removed', 'page', '1', 'throttled', now(), now()), ('" + accountA + "', 'run-kept', 'page', '1', 'throttled', now(), now())",
		"DELETE FROM messages WHERE account_id = '" + accountA + "' AND message_id = 'm-removed'",
		"DELETE FROM job_runs WHERE account_id = '" + accountA + "' AND run_id = 'run-removed'",
	} {
		if code := attempt(t, tx, sql); code != "" {
			t.Fatalf("%s: %s", sql, code)
		}
	}
	for _, table := range []string{"scan_gate_decisions", "job_run_events", "job_run_failures"} {
		if n := countOf(t, tx, table, accountA); n != 1 {
			t.Errorf("%s holds %d of the account's rows after its parent's removal, want the kept parent's 1", table, n)
		}
	}
	var decided string
	if err := tx.QueryRow(t.Context(), "SELECT message_id FROM scan_gate_decisions WHERE account_id = $1", accountA).Scan(&decided); err != nil || decided != "m-kept" {
		t.Errorf("the decision left is %q with error %v, want m-kept's", decided, err)
	}
}

// TestAnAccountWithEvidenceCannotBeDeleted is F2's row for the audit log's key (ADR-0016). Evidence
// outlives everything, so deleting an account that has an audit row is refused, and the row stays.
// An account with no row of any kind is deleted, so the refusal is the audit row's.
func TestAnAccountWithEvidenceCannotBeDeleted(t *testing.T) {
	tx := seeded(t)
	if code := attempt(t, tx, "DELETE FROM accounts WHERE account_id = $1", accountB); code != "" {
		t.Fatalf("deleting an account with no rows: %q, want it deleted", code)
	}
	for _, sql := range []string{
		"DELETE FROM policy_candidates WHERE account_id = '" + accountA + "'",
		"DELETE FROM reorg_op_log",
		"DELETE FROM reorg_plans WHERE account_id = '" + accountA + "'",
	} {
		if code := attempt(t, tx, sql); code != "" {
			t.Fatalf("%s: %s", sql, code)
		}
	}
	if code := attempt(t, tx, "DELETE FROM accounts WHERE account_id = $1", accountA); code != restrictViolation {
		t.Errorf("deleting an account with an audit row: %q, want %s", code, restrictViolation)
	}
	if n := countOf(t, tx, "audit_log", accountA); n != 1 {
		t.Errorf("the account's audit rows are %d, want its 1 kept", n)
	}
}
