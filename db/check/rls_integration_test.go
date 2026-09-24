//go:build integration

package check_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

// rowSecurityRole stands in for any runtime role. The policies apply to every role but the tables'
// owner, and no runtime role yet holds the writes the test needs, so it holds reads and writes on
// every table and nothing else.
const rowSecurityRole = "check_row_security"

// The accounts of the row-level security test. accountB owns the rows. accountC has an account row
// and no rate-state row, and accountD has neither, so each can receive the one row per account those
// two tables hold.
const (
	accountC = "acct-c"
	accountD = "acct-d"
	planB    = "00000000-0000-0000-0000-00000000000b"
)

// accountTable is how the test writes a row of one account-keyed table. insert takes the account as
// $1 and a key that keeps rows apart as $2. owner is the account whose new row the insert writes.
type accountTable struct {
	insert string
	owner  string
}

// accountTables holds every table with an account column. TestRowLevelSecurityScopesEveryAccountTable
// requires it to match the tables the chain holds, so a table added later must be added here.
var accountTables = map[string]accountTable{
	"accounts":            {"INSERT INTO accounts (account_id, provider) VALUES ($1, $2)", accountD},
	"rate_state":          {"INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, classes) VALUES ($1, 1, 1, 1, jsonb_build_object('key', $2::text))", accountC},
	"senders":             {"INSERT INTO senders (account_id, domain) VALUES ($1, $2)", accountB},
	"messages":            {"INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class) VALUES ($1, $2, 't', 'a@example.com', 'example.com', now(), false, 'normal')", accountB},
	"scan_gate_decisions": {"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ($1, $2, 'SCAN', 'restricted')", accountB},
	"policy_candidates":   {"INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ($1, $2, '[]', 0.5)", accountB},
	"policy_rules":        {"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ($1, $1 || '.' || $2, 'restricted', '{example.com}', 'operator', 'operator')", accountB},
	"masking_events":      {"INSERT INTO masking_events (account_id, message_id, field, rule_id, tier) VALUES ($1, $2, 'subject', 'rule', 1)", accountB},
	"reorg_plans":         {"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), $1, 'DRAFT', jsonb_build_object('key', $2::text))", accountB},
	"reorg_plan_ops":      {"INSERT INTO reorg_plan_ops (account_id, plan_id, message_id) VALUES ($1, '" + planB + "', $2)", accountB},
	"job_runs":            {"INSERT INTO job_runs (account_id, run_id, workload, state, started_at) VALUES ($1, $2, 'sync', 'running', now())", accountB},
	"job_run_events":      {"INSERT INTO job_run_events (account_id, run_id, kind) VALUES ($1, $2, 'start')", accountB},
	"job_run_failures":    {"INSERT INTO job_run_failures (account_id, run_id, item_kind, item_id, error_class, first_at, last_at) VALUES ($1, $2, 'page', '1', 'throttled', now(), now())", accountB},
	"audit_log":           {"INSERT INTO audit_log (account_id, actor, action) VALUES ($1, $2, 'READ_BODY')", accountB},
}

// TestRowLevelSecurityScopesEveryAccountTable is ADR-0016's third layer, on every table with an
// account column. Under account A, a row of account B is not read, an update of it changes nothing,
// and a new row for B is refused. Under B the same statements read, update and write B's rows, so
// each result under A is the policy's doing rather than a statement that reaches nothing. A base
// policy rule, which has no account, is read under any account.
func TestRowLevelSecurityScopesEveryAccountTable(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)

	var keyed []string
	rows, err := tx.Query(ctx, `
		SELECT c.table_name FROM information_schema.columns c
		JOIN pg_tables p ON p.schemaname = c.table_schema AND p.tablename = c.table_name
		WHERE c.table_schema = 'public' AND c.column_name = 'account_id' ORDER BY c.table_name`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		keyed = append(keyed, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := slices.Sorted(maps.Keys(accountTables)); !slices.Equal(keyed, want) {
		t.Fatalf("the chain's tables with an account column are %v, and the test covers %v", keyed, want)
	}

	setup := []string{
		"INSERT INTO accounts (account_id, provider) VALUES ('" + accountC + "', 'gmail')",
		"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES ('" + planB + "', '" + accountB + "', 'DRAFT', '{}')",
		"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES (NULL, 'base.example.org', 'restricted', '{example.org}', 'operator', 'operator')",
		"CREATE ROLE " + rowSecurityRole + " NOLOGIN",
		"GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO " + rowSecurityRole,
		"GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO " + rowSecurityRole,
	}
	for _, sql := range setup {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	for _, table := range keyed {
		if table == "accounts" {
			continue
		}
		if _, err := tx.Exec(ctx, accountTables[table].insert, accountB, "seed"); err != nil {
			t.Fatalf("seeding %s: %v", table, err)
		}
	}

	count := func(account, table string) (n int, err error) {
		err = asRole(ctx, tx, rowSecurityRole, account, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()+" WHERE account_id = $1", accountB).Scan(&n)
		})
		return n, err
	}
	for _, table := range keyed {
		quoted := pgx.Identifier{table}.Sanitize()
		update := "UPDATE " + quoted + " SET account_id = account_id WHERE account_id = $1"
		insert := accountTables[table]
		t.Run(table+"/read by another account", func(t *testing.T) {
			if n, err := count(accountA, table); err != nil || n != 0 {
				t.Errorf("read %d of account B's rows with error %v, want none and no error", n, err)
			}
		})
		t.Run(table+"/read by the owning account", func(t *testing.T) {
			if n, err := count(accountB, table); err != nil || n == 0 {
				t.Errorf("read %d of account B's rows with error %v, want some and no error", n, err)
			}
		})
		t.Run(table+"/update by another account", func(t *testing.T) {
			if n, err := as(ctx, tx, rowSecurityRole, accountA, update, accountB); err != nil || n != 0 {
				t.Errorf("updated %d of account B's rows with error %v, want none and no error", n, err)
			}
		})
		t.Run(table+"/update by the owning account", func(t *testing.T) {
			if n, err := as(ctx, tx, rowSecurityRole, accountB, update, accountB); err != nil || n == 0 {
				t.Errorf("updated %d of account B's rows with error %v, want some and no error", n, err)
			}
		})
		t.Run(table+"/insert by another account", func(t *testing.T) {
			if _, err := as(ctx, tx, rowSecurityRole, accountA, insert.insert, insert.owner, "new"); !refusedByPolicy(err) {
				t.Errorf("inserting a row of %s: got %v, want the policy to refuse it", insert.owner, err)
			}
		})
		t.Run(table+"/insert by the owning account", func(t *testing.T) {
			if n, err := as(ctx, tx, rowSecurityRole, insert.owner, insert.insert, insert.owner, "new"); err != nil || n != 1 {
				t.Errorf("inserted %d rows of %s with error %v, want 1 and no error", n, insert.owner, err)
			}
		})
	}
	t.Run("policy_rules/read a base rule", func(t *testing.T) {
		var n int
		err := asRole(ctx, tx, rowSecurityRole, accountA, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM policy_rules WHERE account_id IS NULL").Scan(&n)
		})
		if err != nil || n != 1 {
			t.Errorf("read %d base rules with error %v, want 1 and no error", n, err)
		}
	})
}

// TestAPolicyRuleWriteNamesTheTransactionsAccount writes base rules, which carry no account and bind
// every account (ADR-0004). A runtime role reads them but writes only its own account's rules, so the
// UI's insert grant reaches no further than the rule its account's confirmation emits (ADR-0021).
func TestAPolicyRuleWriteNamesTheTransactionsAccount(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES (NULL, 'base.example.org', 'restricted', '{example.org}', 'operator', 'operator')",
		"CREATE ROLE " + rowSecurityRole + " NOLOGIN",
		"GRANT SELECT, UPDATE ON policy_rules TO " + rowSecurityRole,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	const insert = "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ($1, $2, 'restricted', '{example.com}', 'candidate', 'operator')"
	t.Run("the UI inserts its account's rule", func(t *testing.T) {
		if n, err := as(ctx, tx, uiRole, accountA, insert, accountA, "candidate.acct-a.example.com"); err != nil || n != 1 {
			t.Errorf("inserted %d rules with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("the UI inserts a base rule", func(t *testing.T) {
		if _, err := as(ctx, tx, uiRole, accountA, insert, nil, "base.example.com"); !refusedByPolicy(err) {
			t.Errorf("got %v, want the policy to refuse a rule with no account", err)
		}
	})
	t.Run("the UI reads the base rule", func(t *testing.T) {
		var n int
		err := asRole(ctx, tx, uiRole, accountA, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM policy_rules WHERE account_id IS NULL").Scan(&n)
		})
		if err != nil || n != 1 {
			t.Errorf("read %d base rules with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("a role updates a base rule", func(t *testing.T) {
		if n, err := as(ctx, tx, rowSecurityRole, accountA, "UPDATE policy_rules SET class = class WHERE account_id IS NULL"); err != nil || n != 0 {
			t.Errorf("updated %d base rules with error %v, want none and no error", n, err)
		}
	})
}
