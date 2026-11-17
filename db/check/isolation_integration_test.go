//go:build integration

package check_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// isolationRole stands in for any user a program connects as. It holds reads and writes on every table and nothing
// else, so what each statement below reaches is the statement's own doing.
const isolationRole = "check_isolation"

// The accounts of the isolation test. accountB owns the rows. accountC has an account row and no
// rate-state or state row, and accountD and accountE have none of them, so each can receive the one row
// per account those tables hold.
const (
	accountC = "acct-c"
	accountD = "acct-d"
	accountE = "acct-e"
	planB    = "00000000-0000-0000-0000-00000000000b"
)

// accountTable is how the test writes a row of one account-keyed table. insert takes the account as
// $1 and a key that keeps rows apart as $2. owner is the account whose new row the insert writes.
type accountTable struct {
	insert string
	owner  string
}

// accountTables holds every table with an account column. TestTheTransactionsAccountScopesEveryAccountTable
// requires it to match the tables the chain holds, so a table added later must be added here.
var accountTables = map[string]accountTable{
	"accounts":            {"INSERT INTO accounts (account_id, provider) VALUES ($1, $2)", accountD},
	"account_state":       {"INSERT INTO account_state (account_id, sync_cursor) VALUES ($1, $2)", accountC},
	"rate_grants":         {"INSERT INTO rate_grants (account_id, class, tokens, issued_at) VALUES ($1, 'batch', length($2::text), now())", accountB},
	"rate_state":          {"INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, classes) VALUES ($1, 1, 1, 1, jsonb_build_object('key', $2::text))", accountC},
	"senders":             {"INSERT INTO senders (account_id, domain) VALUES ($1, $2)", accountB},
	"messages":            {"INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments) VALUES ($1, $2, 't', 'a@example.com', 'example.com', now(), false)", accountB},
	"attachment_media":    {"INSERT INTO attachment_media (account_id, message_id, media_type, extension) VALUES ($1, 'm-media', '', $2)", accountB},
	"scan_gate_decisions": {"INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason) VALUES ($1, 'm-' || $2, 'SCAN', 'restricted')", accountB},
	"policy_candidates":   {"INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ($1, $2, '[]', 0.5)", accountB},
	"policy_changes":      {"INSERT INTO policy_changes (account_id, actor, action, rule_id) VALUES ($1, 'operator', 'added', $2)", accountB},
	"policy_rules":        {"INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ($1, $1 || '.' || $2, 'restricted', '{example.com}', 'operator', 'operator')", accountB},
	"masking_events":      {"INSERT INTO masking_events (account_id, message_id, field, rule_id, tier) VALUES ($1, $2, 'subject', 'rule', 1)", accountB},
	"reorg_plans":         {"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), $1, 'DRAFT', jsonb_build_object('key', $2::text))", accountB},
	"reorg_plan_ops":      {"INSERT INTO reorg_plan_ops (account_id, plan_id, message_id) VALUES ($1, '" + planB + "', $2)", accountB},
	"job_runs":            {"INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) VALUES ($1, $2, 'sync', 'tick', 'running', now())", accountB},
	"job_run_events":      {"INSERT INTO job_run_events (account_id, run_id, kind, detail) VALUES ($1, 'run-' || $1, 'start', $2)", accountB},
	"job_run_failures":    {"INSERT INTO job_run_failures (account_id, run_id, item_kind, item_id, error_class, first_at, last_at) VALUES ($1, 'run-' || $1, 'page', $2, 'throttled', now(), now())", accountB},
	"audit_log":           {"INSERT INTO audit_log (account_id, actor, action, stage, reason, sender_class, scan_state) VALUES ($1, $2, 'READ_BODY', 'serve', 'released', 'normal', 'scanned')", accountB},
}

// TestTheTransactionsAccountScopesEveryAccountTable is ADR-0135's form on every table with an account
// column. A statement compares the account column with the transaction's account, and an insert takes
// it from there, so no statement names an account of its own. Under account A such a read reaches none
// of B's rows, an update changes none of them, and an insert writes A's row and never B's. Under B the
// same statements read, update and write B's rows, so each result under A is the setting's doing
// rather than a statement that reaches nothing. The statement check refuses a statement in any other
// form (db/check's account-predicate and account-parameter checks).
func TestTheTransactionsAccountScopesEveryAccountTable(t *testing.T) {
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
		"CREATE ROLE " + isolationRole + " NOLOGIN",
		// The parents the foreign keys require, a message of each account for its attachment media and
		// gate decision, and a run of each for its events and failures.
		"INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments) " +
			"SELECT a, m, 't', 'a@example.com', 'example.com', now(), true FROM (VALUES ('" + accountA + "', 'm-media'), ('" + accountB + "', 'm-media'), ('" +
			accountA + "', 'm-new'), ('" + accountB + "', 'm-seed'), ('" + accountB + "', 'm-new')) AS v (a, m)",
		"INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at) " +
			"SELECT a, 'run-' || a, 'sync', 'tick', 'running', now() FROM (VALUES ('" + accountA + "'), ('" + accountB + "')) AS v (a)",
		"GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO " + isolationRole,
		"GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO " + isolationRole,
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

	owned := func(q interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}, table, account string,
	) (n int, err error) {
		err = q.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()+" WHERE account_id = $1", account).Scan(&n)
		return n, err
	}
	for _, table := range keyed {
		quoted := pgx.Identifier{table}.Sanitize()
		read := "SELECT count(*) FROM " + quoted + " WHERE account_id = current_setting('app.account')"
		update := "UPDATE " + quoted + " SET account_id = account_id WHERE account_id = current_setting('app.account')"
		insert := accountTables[table]
		// The table's own insert, with the account taken from the transaction in place of $1.
		fromTransaction := strings.ReplaceAll(strings.ReplaceAll(insert.insert, "$1", "current_setting('app.account')"), "$2", "$1")
		// The account the insert below runs under, one that can take a new row of the table.
		other := accountA
		if table == "accounts" {
			other = accountE
		}
		for _, account := range []string{accountA, accountB} {
			t.Run(table+"/read and update under "+account, func(t *testing.T) {
				want, err := owned(tx, table, account)
				if err != nil {
					t.Fatal(err)
				}
				if account == accountB && want == 0 {
					t.Fatal("account B owns no row, so the case below would prove nothing")
				}
				var n int
				if err := asRole(ctx, tx, isolationRole, account, func(sp pgx.Tx) error { return sp.QueryRow(ctx, read).Scan(&n) }); err != nil || n != want {
					t.Errorf("read %d rows with error %v, want the account's own %d and no error", n, err, want)
				}
				if updated, err := as(ctx, tx, isolationRole, account, update); err != nil || updated != int64(want) {
					t.Errorf("updated %d rows with error %v, want the account's own %d and no error", updated, err, want)
				}
			})
		}
		t.Run(table+"/insert under another account", func(t *testing.T) {
			before, err := owned(tx, table, insert.owner)
			if err != nil {
				t.Fatal(err)
			}
			var ownerAfter, otherAfter, otherBefore int
			err = asRole(ctx, tx, isolationRole, other, func(sp pgx.Tx) error {
				if otherBefore, err = owned(sp, table, other); err != nil {
					return err
				}
				if _, err := sp.Exec(ctx, fromTransaction, "new"); err != nil {
					return err
				}
				if ownerAfter, err = owned(sp, table, insert.owner); err != nil {
					return err
				}
				otherAfter, err = owned(sp, table, other)
				return err
			})
			if err != nil || ownerAfter != before || otherAfter != otherBefore+1 {
				t.Errorf("an insert under %s left %s with %d rows rather than %d and %s with %d rather than %d, error %v, want the row written for %s alone",
					other, insert.owner, ownerAfter, before, other, otherAfter, otherBefore+1, err, other)
			}
		})
		t.Run(table+"/insert under the owning account", func(t *testing.T) {
			if n, err := as(ctx, tx, isolationRole, insert.owner, fromTransaction, "new"); err != nil || n != 1 {
				t.Errorf("inserted %d rows of %s with error %v, want 1 and no error", n, insert.owner, err)
			}
		})
	}
}

// TestTheApplicationUserListsEveryAccountAndReadsOneState is ADR-0091's listing and ADR-0135's state
// read as the application user, as the migration chain grants it. On a connection that never set the
// account, the listing reads every account's identifier, provider and client, since it compares no
// account. With the transaction set to one account, a read of account_state in the statements' form
// returns that account's row alone, its sealed credential included, and an update of the credential in
// that form changes that row alone. What a program's code may read of an account's state is held by its
// import list, since one user's grants cannot tell programs apart (ADR-0143).
func TestTheApplicationUserListsEveryAccountAndReadsOneState(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if _, err := tx.Exec(ctx, "INSERT INTO account_state (account_id, credential) VALUES ($1, 'a'), ($2, 'b')", accountA, accountB); err != nil {
		t.Fatal(err)
	}
	t.Run("the listing", func(t *testing.T) {
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := sp.Rollback(context.Background()); err != nil {
				t.Error(err)
			}
		}()
		if _, err := sp.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{appUser}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := sp.QueryRow(ctx, "SELECT count(*) FROM (SELECT account_id, provider, oauth_client FROM accounts) AS a").Scan(&n); err != nil || n != 2 {
			t.Errorf("listed %d of the two accounts with error %v, want both and no error", n, err)
		}
	})
	t.Run("the state read", func(t *testing.T) {
		var accounts []string
		err := asRole(ctx, tx, appUser, accountA, func(sp pgx.Tx) error {
			rows, err := sp.Query(ctx, "SELECT account_id FROM account_state WHERE account_id = current_setting('app.account') AND credential IS NOT NULL")
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
	t.Run("the credential's write-back", func(t *testing.T) {
		if n, err := as(ctx, tx, appUser, accountA, "UPDATE account_state SET credential = 'x' WHERE account_id = current_setting('app.account')"); err != nil || n != 1 {
			t.Errorf("updated %d rows with error %v, want its own account's alone and no error", n, err)
		}
	})
}

// TestTheApplicationUserReadsAndWritesTheOAuthClients holds oauth_clients' grants. An OAuth client
// belongs to no account (ADR-0016), so the application user reads every row, its sealed secret
// included, which the connections and the UI's one part that opens a client's secret read (ADR-0081),
// adds, replaces and removes a client, which the UI's client setup does (ADR-0084), and rewrites a
// client's secret, which the connections' re-seal does (ADR-0092). Which code may do each is held by the
// import lists (ADR-0143). No statement changes a client's name or provider, so the user is refused it.
func TestTheApplicationUserReadsAndWritesTheOAuthClients(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if _, err := tx.Exec(ctx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'id', 's'), ('other', 'other', 'id', 's')"); err != nil {
		t.Fatal(err)
	}
	t.Run("reads every client", func(t *testing.T) {
		var n int
		err := asRole(ctx, tx, appUser, accountA, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM (SELECT name, provider, client_id, client_secret, project_id FROM oauth_clients) AS c").Scan(&n)
		})
		if err != nil || n != 2 {
			t.Errorf("read %d clients with error %v, want both and no error", n, err)
		}
	})
	t.Run("sets up, re-seals and removes a client", func(t *testing.T) {
		err := asRole(ctx, tx, appUser, accountA, func(sp pgx.Tx) error {
			for _, sql := range []string{
				"INSERT INTO oauth_clients (name, provider, client_id, client_secret, project_id) VALUES ('new', 'gmail', 'new-id', 's', 'project')",
				"UPDATE oauth_clients SET client_id = 'id-2', client_secret = 's-2', project_id = NULL WHERE name = 'new'",
				"UPDATE oauth_clients SET client_secret = 'resealed' WHERE name = 'household' AND client_secret = 's'",
				"DELETE FROM oauth_clients WHERE name = 'other'",
			} {
				tag, err := sp.Exec(ctx, sql)
				if err != nil {
					return fmt.Errorf("%s: %w", sql, err)
				}
				if tag.RowsAffected() != 1 {
					return fmt.Errorf("%s wrote %d rows, want 1", sql, tag.RowsAffected())
				}
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("changes a client's name or provider", func(t *testing.T) {
		for _, sql := range []string{"UPDATE oauth_clients SET name = name", "UPDATE oauth_clients SET provider = provider"} {
			if _, err := as(ctx, tx, appUser, accountA, sql); !refusedByGrant(err) {
				t.Errorf("%s: got %v, want the grant to refuse it", sql, err)
			}
		}
	})
}
