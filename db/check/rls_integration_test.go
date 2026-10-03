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
	"policy_changes":      {"INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES ($1, 'operator', 'added', $2)", accountB},
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
// every account (ADR-0004), in an account's transaction. A runtime role reads them there but writes
// only its own account's rules, so the UI's grant reaches the base rules only in a base-policy
// transaction, which TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction covers (ADR-0112).
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

// asBase runs fn as role in a savepoint of tx, set as the transaction helper's base-policy entry point
// sets a transaction, the account empty and app.base on, with extra settings applied after them, and
// rolls the savepoint back (ADR-0112).
func asBase(ctx context.Context, tx pgx.Tx, role string, extra map[string]string, fn func(pgx.Tx) error) (err error) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := sp.Rollback(context.Background()); err == nil {
			err = rollbackErr
		}
	}()
	for _, sql := range []string{
		"SET LOCAL ROLE " + pgx.Identifier{role}.Sanitize(),
		"SELECT set_config('app.account', '', true)",
		"SELECT set_config('app.base', 'on', true)",
	} {
		if _, err := sp.Exec(ctx, sql); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(extra)) {
		if _, err := sp.Exec(ctx, "SELECT set_config($1, $2, true)", name, extra[name]); err != nil {
			return err
		}
	}
	return fn(sp)
}

// inBase runs one statement through asBase and returns the rows it affected.
func inBase(ctx context.Context, tx pgx.Tx, role string, extra map[string]string, sql string, args ...any) (affected int64, err error) {
	err = asBase(ctx, tx, role, extra, func(sp pgx.Tx) error {
		tag, err := sp.Exec(ctx, sql, args...)
		affected = tag.RowsAffected()
		return err
	})
	return affected, err
}

// TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction is ADR-0112's scope rule. The UI's role
// writes a base rule and its history row in a base-policy transaction, and nowhere else. In an
// account's transaction it writes neither, in a base-policy transaction it reads and writes no
// account's rule or history row, a transaction that set both an account and app.base writes no base
// row, and no other role writes the base policy in any transaction. The successful writes come first
// in each pairing, so each refusal is the policy's doing rather than a statement that reaches nothing.
func TestTheBasePolicyIsWrittenOnlyInABasePolicyTransaction(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES (NULL, 'base.example.org', 'restricted', '{example.org}', 'operator', 'operator')",
		"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ('" + accountA + "', 'own.example.net', 'restricted', '{example.net}', 'operator', 'operator')",
		"INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES (NULL, 'operator', 'added', 'base.example.org')",
		"INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES ('" + accountA + "', 'operator', 'added', 'own.example.net')",
		"CREATE ROLE " + rowSecurityRole + " NOLOGIN",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON policy_rules, policy_changes TO " + rowSecurityRole,
		"GRANT USAGE ON SEQUENCE policy_changes_id_seq TO " + rowSecurityRole,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	const (
		insertRule   = "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ($1, 'new.example.com', 'restricted', '{example.com}', 'operator', 'operator')"
		insertChange = "INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES ($1, 'operator', 'added', 'new.example.com')"
		editBase     = "UPDATE policy_rules SET domain_suffix = '{example.org,example.info}' WHERE account_id IS NULL"
		liftBase     = "DELETE FROM policy_rules WHERE account_id IS NULL"
		editOwn      = "UPDATE policy_rules SET domain_suffix = '{example.net,example.info}' WHERE account_id = '" + accountA + "'"
		liftOwn      = "DELETE FROM policy_rules WHERE account_id = '" + accountA + "'"
	)
	written := func(t *testing.T, name string, n int64, err error) {
		t.Helper()
		if err != nil || n != 1 {
			t.Errorf("%s wrote %d rows with error %v, want 1 and no error", name, n, err)
		}
	}
	untouched := func(t *testing.T, name string, n int64, err error) {
		t.Helper()
		if err != nil || n != 0 {
			t.Errorf("%s changed %d rows with error %v, want none and no error", name, n, err)
		}
	}
	refused := func(t *testing.T, name string, err error) {
		t.Helper()
		if !refusedByPolicy(err) {
			t.Errorf("%s: got %v, want the policy to refuse the row", name, err)
		}
	}
	t.Run("the UI writes the base policy in a base-policy transaction", func(t *testing.T) {
		n, err := inBase(ctx, tx, uiRole, nil, insertRule, nil)
		written(t, "inserting a base rule", n, err)
		n, err = inBase(ctx, tx, uiRole, nil, insertChange, nil)
		written(t, "inserting a base history row", n, err)
		n, err = inBase(ctx, tx, uiRole, nil, editBase)
		written(t, "editing a base rule", n, err)
		n, err = inBase(ctx, tx, uiRole, nil, liftBase)
		written(t, "lifting a base rule", n, err)
	})
	t.Run("the UI writes no base row in an account's transaction", func(t *testing.T) {
		_, err := as(ctx, tx, uiRole, accountA, insertRule, nil)
		refused(t, "inserting a base rule", err)
		_, err = as(ctx, tx, uiRole, accountA, insertChange, nil)
		refused(t, "inserting a base history row", err)
		n, err := as(ctx, tx, uiRole, accountA, editBase)
		untouched(t, "editing a base rule", n, err)
		n, err = as(ctx, tx, uiRole, accountA, liftBase)
		untouched(t, "lifting a base rule", n, err)
	})
	t.Run("the UI writes its account's rule in its account's transaction", func(t *testing.T) {
		n, err := as(ctx, tx, uiRole, accountA, editOwn)
		written(t, "editing the account's rule", n, err)
		n, err = as(ctx, tx, uiRole, accountA, liftOwn)
		written(t, "lifting the account's rule", n, err)
	})
	t.Run("a base-policy transaction reaches no account's rows", func(t *testing.T) {
		_, err := inBase(ctx, tx, uiRole, nil, insertRule, accountA)
		refused(t, "inserting an account's rule", err)
		_, err = inBase(ctx, tx, uiRole, nil, insertChange, accountA)
		refused(t, "inserting an account's history row", err)
		n, err := inBase(ctx, tx, uiRole, nil, editOwn)
		untouched(t, "editing an account's rule", n, err)
		n, err = inBase(ctx, tx, uiRole, nil, liftOwn)
		untouched(t, "lifting an account's rule", n, err)
		var rules, changes, base int
		err = asBase(ctx, tx, uiRole, nil, func(sp pgx.Tx) error {
			if err := sp.QueryRow(ctx, "SELECT count(*) FROM policy_rules WHERE account_id IS NOT NULL").Scan(&rules); err != nil {
				return err
			}
			if err := sp.QueryRow(ctx, "SELECT count(*) FROM policy_changes WHERE account_id IS NOT NULL").Scan(&changes); err != nil {
				return err
			}
			return sp.QueryRow(ctx, "SELECT count(*) FROM policy_rules WHERE account_id IS NULL").Scan(&base)
		})
		if err != nil || rules != 0 || changes != 0 || base != 1 {
			t.Errorf("read %d account rules, %d account history rows and %d base rules with error %v, want 0, 0 and 1", rules, changes, base, err)
		}
	})
	t.Run("a transaction naming an account and the base policy writes no base row", func(t *testing.T) {
		both := map[string]string{"app.account": accountA}
		_, err := inBase(ctx, tx, uiRole, both, insertRule, nil)
		refused(t, "inserting a base rule", err)
		n, err := inBase(ctx, tx, uiRole, both, editBase)
		untouched(t, "editing a base rule", n, err)
	})
	t.Run("no other role writes the base policy", func(t *testing.T) {
		_, err := inBase(ctx, tx, rowSecurityRole, nil, insertRule, nil)
		refused(t, "inserting a base rule", err)
		_, err = inBase(ctx, tx, rowSecurityRole, nil, insertChange, nil)
		refused(t, "inserting a base history row", err)
		n, err := inBase(ctx, tx, rowSecurityRole, nil, editBase)
		untouched(t, "editing a base rule", n, err)
		n, err = inBase(ctx, tx, rowSecurityRole, nil, liftBase)
		untouched(t, "lifting a base rule", n, err)
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

// syncRole is delta sync's role, the one runtime role that writes a client's secret (ADR-0092).
const syncRole = "mediated_mailbox_sync"

// TestTheProviderCallingRolesReadOnlyTheirAccountsState holds the account snapshot's grants on
// account_state to row-level security, under the roles as the migration chain grants them. Each of the
// four roles that call a provider reads its transaction's account's credential and no other's, and an
// update of another account's credential changes nothing. The UI's role reads its transaction's
// account's progress and last authentication, the columns its statement in db/accountstate reads,
// and never a credential. No other role reads the table (ADR-0084, ADR-0091).
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
	t.Run(uiRole+"/reads its own account's progress", func(t *testing.T) {
		var accounts []string
		err := asRole(ctx, tx, uiRole, accountA, func(sp pgx.Tx) error {
			rows, err := sp.Query(ctx, `SELECT account_id FROM account_state
				WHERE NOT backfill_pass1_complete AND NOT backfill_pass2_complete
				AND sync_cursor_at IS NULL AND last_auth_at IS NULL AND last_auth_outcome IS NULL ORDER BY account_id`)
			if err != nil {
				return err
			}
			accounts, err = pgx.CollectRows(rows, pgx.RowTo[string])
			return err
		})
		if err != nil || !slices.Equal(accounts, []string{accountA}) {
			t.Errorf("read the progress of %v with error %v, want %s's alone", accounts, err, accountA)
		}
	})
	t.Run(uiRole+"/reads a credential", func(t *testing.T) {
		if _, err := as(ctx, tx, uiRole, accountA, "SELECT credential FROM account_state"); !refusedByGrant(err) {
			t.Errorf("got %v, want the grant to refuse it", err)
		}
	})
	t.Run("mediated_mailbox_propose/reads account state", func(t *testing.T) {
		if _, err := as(ctx, tx, "mediated_mailbox_propose", accountA, "SELECT account_id FROM account_state"); !refusedByGrant(err) {
			t.Errorf("got %v, want the grant to refuse it", err)
		}
	})
}

// TestOnlyTheProviderCallingRolesReadTheOAuthClients holds oauth_clients' grants, the only barrier
// around a table that belongs to no account (ADR-0016). The four roles that call a provider read every
// row, each client's name included (ADR-0106). The UI's role reads each client's name, provider,
// identifier, project ID and sealed secret, which the one part of the UI that opens a client's secret
// opens for a consent's code exchange (ADR-0081), and writes what its client setup writes, adding a
// client, replacing its identifier, secret and project ID, and removing it (ADR-0084). Delta sync's
// role writes a client's secret, which its re-seal does, and nothing else of the table. No other role
// reads it or writes it (ADR-0092).
func TestOnlyTheProviderCallingRolesReadTheOAuthClients(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if _, err := tx.Exec(ctx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'id', 's'), ('other', 'other', 'id', 's')"); err != nil {
		t.Fatal(err)
	}
	for _, role := range providerRoles {
		t.Run(role+"/reads every client", func(t *testing.T) {
			var n int
			err := asRole(ctx, tx, role, accountA, func(sp pgx.Tx) error {
				return sp.QueryRow(ctx, "SELECT count(*) FROM (SELECT name, provider, client_id, client_secret FROM oauth_clients) AS c").Scan(&n)
			})
			if err != nil || n != 2 {
				t.Errorf("read %d clients with error %v, want both and no error", n, err)
			}
		})
	}
	t.Run(uiRole+"/reads every client", func(t *testing.T) {
		var n int
		err := asRole(ctx, tx, uiRole, accountA, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM (SELECT name, provider, client_id, client_secret, project_id FROM oauth_clients) AS c").Scan(&n)
		})
		if err != nil || n != 2 {
			t.Errorf("read %d clients with error %v, want both and no error", n, err)
		}
	})
	t.Run("mediated_mailbox_propose/reads a client", func(t *testing.T) {
		// Each column is read on its own, so a grant of any one of them is caught.
		for _, column := range []string{"name", "provider", "client_id", "client_secret", "project_id"} {
			if _, err := as(ctx, tx, "mediated_mailbox_propose", accountA, "SELECT "+column+" FROM oauth_clients"); !refusedByGrant(err) {
				t.Errorf("reading %s: got %v, want the grant to refuse it", column, err)
			}
		}
	})
	for _, role := range runtimeRoles {
		t.Run(role+"/writes a client", func(t *testing.T) {
			_, err := as(ctx, tx, role, accountA, "UPDATE oauth_clients SET client_secret = 'resealed' WHERE name = 'household'")
			switch {
			case (role == syncRole || role == uiRole) && err != nil:
				t.Errorf("updating the secret: got %v, want it written", err)
			case role != syncRole && role != uiRole && !refusedByGrant(err):
				t.Errorf("updating the secret: got %v, want the grant to refuse it", err)
			}
			writes := map[string]string{
				"updating the identifier": "UPDATE oauth_clients SET client_id = client_id",
				"inserting":               "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('new', 'gmail', 'new-id', 's')",
				"deleting":                "DELETE FROM oauth_clients WHERE name = 'other'",
			}
			for _, name := range slices.Sorted(maps.Keys(writes)) {
				_, err := as(ctx, tx, role, accountA, writes[name])
				switch {
				case role == uiRole && err != nil:
					t.Errorf("%s: got %v, want the UI's client setup to write it", name, err)
				case role != uiRole && !refusedByGrant(err):
					t.Errorf("%s: got %v, want the grant to refuse it", name, err)
				}
			}
			if _, err := as(ctx, tx, role, accountA, "UPDATE oauth_clients SET name = name, provider = provider"); !refusedByGrant(err) {
				t.Errorf("updating the name and provider: got %v, want the grant to refuse it", err)
			}
		})
	}
}
