//go:build integration

package check_test

import (
	"context"
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
// and no rate-state or state row, and accountD has none of them, so each can receive the one row per
// account those tables hold.
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
	"account_state":       {"INSERT INTO account_state (account_id, sync_cursor) VALUES ($1, $2)", accountC},
	"rate_grants":         {"INSERT INTO rate_grants (account_id, class, tokens, issued_at) VALUES ($1, $2, 1, now())", accountB},
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
// policy rule, which has no account, is read under any account. The role here is none of the roles
// that list accounts, so the accounts table is confined for it like every other table (ADR-0091).
// TestTheListingRolesReadEveryAccount covers the listing roles.
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

// listingRoles are the roles that read every row of accounts, the UI's for its account selector and
// the provider-calling deployables' for their account snapshots (ADR-0091). The heuristics job's role
// is not among them.
var listingRoles = []string{
	"mediated_mailbox_backfill",
	"mediated_mailbox_mediate",
	"mediated_mailbox_organize",
	"mediated_mailbox_sync",
	uiRole,
}

// TestTheListingRolesReadEveryAccount is ADR-0091's exception to ADR-0016's third layer. Each role that
// lists accounts reads every account's identifier and provider, even with no account set, while an
// insert or update of another account's row is refused or changes nothing. A runtime role outside the
// list sees only its transaction's account. The grants here are the test's own, made in the
// transaction it rolls back, since each role's grant on accounts arrives with the statement that
// needs it.
func TestTheListingRolesReadEveryAccount(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	const outside = "mediated_mailbox_propose"
	for _, role := range append(slices.Clone(listingRoles), outside) {
		if _, err := tx.Exec(ctx, "GRANT SELECT, INSERT, UPDATE ON accounts TO "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	listed := func(role, account string) (n int, err error) {
		err = asRole(ctx, tx, role, account, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE account_id IN ($1, $2)", accountA, accountB).Scan(&n)
		})
		return n, err
	}
	for _, role := range listingRoles {
		t.Run(role+"/lists every account", func(t *testing.T) {
			if n, err := listed(role, accountA); err != nil || n != 2 {
				t.Errorf("listed %d of the two accounts with error %v, want both and no error", n, err)
			}
		})
		t.Run(role+"/updates another account's row", func(t *testing.T) {
			if n, err := as(ctx, tx, role, accountA, "UPDATE accounts SET provider = provider WHERE account_id = $1", accountB); err != nil || n != 0 {
				t.Errorf("updated %d of account B's rows with error %v, want none and no error", n, err)
			}
		})
		t.Run(role+"/inserts another account's row", func(t *testing.T) {
			if _, err := as(ctx, tx, role, accountA, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", "acct-new"); !refusedByPolicy(err) {
				t.Errorf("got %v, want the policy to refuse it", err)
			}
		})
	}
	t.Run(outside+"/lists only its own account", func(t *testing.T) {
		if n, err := listed(outside, accountA); err != nil || n != 1 {
			t.Errorf("listed %d of the two accounts with error %v, want its own alone and no error", n, err)
		}
	})
}

// TestAListingNeedsNoAccountSet runs a listing as each listing role on a connection that never set
// the account, where the per-account policy's setting would raise if the planner evaluated it
// (ADR-0016). The listing policy makes the per-account one irrelevant to a listing role's read, so
// the listing needs no account. The grant is the test's own, made in the transaction it rolls back.
func TestAListingNeedsNoAccountSet(t *testing.T) {
	for _, role := range listingRoles {
		t.Run(role, func(t *testing.T) {
			ctx := t.Context()
			tx, err := connect(t).Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := tx.Rollback(context.Background()); err != nil {
					t.Error(err)
				}
			}()
			for _, sql := range []string{
				"INSERT INTO accounts (account_id, provider) VALUES ('" + accountA + "', 'gmail'), ('" + accountB + "', 'gmail')",
				"GRANT SELECT ON accounts TO " + pgx.Identifier{role}.Sanitize(),
				"SET LOCAL ROLE " + pgx.Identifier{role}.Sanitize(),
			} {
				if _, err := tx.Exec(ctx, sql); err != nil {
					t.Fatalf("%s: %v", sql, err)
				}
			}
			var n int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE account_id IN ($1, $2)", accountA, accountB).Scan(&n); err != nil || n != 2 {
				t.Errorf("listed %d of the two accounts with error %v, want both and no error", n, err)
			}
		})
	}
}

// providerRoles are the roles of the four deployables that call a provider, which read every OAuth
// client and their own account's state (ADR-0016, ADR-0075, ADR-0091).
var providerRoles = []string{
	"mediated_mailbox_backfill",
	"mediated_mailbox_mediate",
	"mediated_mailbox_organize",
	"mediated_mailbox_sync",
}

// TestTheProviderCallingRolesReadOnlyTheirAccountsState holds the account snapshot's grants on
// account_state to row-level security, under the roles as the migration chain grants them. Each of the
// four roles that call a provider reads its transaction's account's credential and no other's, and an
// update of another account's credential changes nothing. No other role reads the table, the UI's
// included, whose read arrives with the statement that needs it (ADR-0084, ADR-0091).
func TestTheProviderCallingRolesReadOnlyTheirAccountsState(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if _, err := tx.Exec(ctx, "INSERT INTO account_state (account_id, credential) VALUES ($1, 'a'), ($2, 'b')", accountA, accountB); err != nil {
		t.Fatal(err)
	}
	for _, role := range providerRoles {
		t.Run(role+"/reads its own account's state", func(t *testing.T) {
			var accounts []string
			err := asRole(ctx, tx, role, accountA, func(sp pgx.Tx) error {
				rows, err := sp.Query(ctx, "SELECT account_id FROM account_state WHERE credential IS NOT NULL ORDER BY account_id")
				if err != nil {
					return err
				}
				accounts, err = pgx.CollectRows(rows, pgx.RowTo[string])
				return err
			})
			if err != nil || !slices.Equal(accounts, []string{accountA}) {
				t.Errorf("read the state of %v with error %v, want %s's alone", accounts, err, accountA)
			}
		})
		t.Run(role+"/updates another account's credential", func(t *testing.T) {
			if n, err := as(ctx, tx, role, accountA, "UPDATE account_state SET credential = 'x' WHERE account_id = $1", accountB); err != nil || n != 0 {
				t.Errorf("updated %d of account B's rows with error %v, want none and no error", n, err)
			}
		})
	}
	for _, role := range []string{uiRole, "mediated_mailbox_propose"} {
		t.Run(role+"/reads account state", func(t *testing.T) {
			if _, err := as(ctx, tx, role, accountA, "SELECT account_id FROM account_state"); !refusedByGrant(err) {
				t.Errorf("got %v, want the grant to refuse it", err)
			}
		})
	}
}

// TestOnlyTheProviderCallingRolesReadTheOAuthClients holds oauth_clients' grants, the only barrier
// around a table that belongs to no account (ADR-0016). The four roles that call a provider read every
// row. The UI's read of the client's identity arrives with M7's statements, and delta sync's write
// with the statement that re-seals a client's secret, so today no other role reads it and no role
// writes it (ADR-0084, ADR-0092).
func TestOnlyTheProviderCallingRolesReadTheOAuthClients(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if _, err := tx.Exec(ctx, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ('gmail', 'id', 's'), ('other', 'id', 's')"); err != nil {
		t.Fatal(err)
	}
	for _, role := range providerRoles {
		t.Run(role+"/reads every client", func(t *testing.T) {
			var n int
			err := asRole(ctx, tx, role, accountA, func(sp pgx.Tx) error {
				return sp.QueryRow(ctx, "SELECT count(client_secret) FROM oauth_clients").Scan(&n)
			})
			if err != nil || n != 2 {
				t.Errorf("read %d clients with error %v, want both and no error", n, err)
			}
		})
	}
	for _, role := range []string{uiRole, "mediated_mailbox_propose"} {
		t.Run(role+"/reads a client", func(t *testing.T) {
			if _, err := as(ctx, tx, role, accountA, "SELECT provider, client_id FROM oauth_clients"); !refusedByGrant(err) {
				t.Errorf("got %v, want the grant to refuse it", err)
			}
		})
	}
	for _, role := range runtimeRoles {
		t.Run(role+"/writes a client", func(t *testing.T) {
			if _, err := as(ctx, tx, role, accountA, "UPDATE oauth_clients SET client_secret = client_secret"); !refusedByGrant(err) {
				t.Errorf("updating: got %v, want the grant to refuse it", err)
			}
			if _, err := as(ctx, tx, role, accountA, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ('new', 'id', 's')"); !refusedByGrant(err) {
				t.Errorf("inserting: got %v, want the grant to refuse it", err)
			}
		})
	}
}
