//go:build integration

package check_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The tests below run statements under the runtime roles against the migration chain pgrun applied
// from empty to this package's database. Each works inside one transaction it rolls back, and runs
// each statement in a savepoint of its own under the role, so a refused statement leaves the rest of
// the test able to run and nothing reaches the other tests of this package.

// migrationRole owns the schema (ADR-0048).
const migrationRole = "mediated_mailbox_migrate"

// runtimeRoles is one role per deployable, named for its directory (ADR-0075).
var runtimeRoles = []string{
	"mediated_mailbox_backfill",
	"mediated_mailbox_mediate",
	"mediated_mailbox_organize",
	"mediated_mailbox_propose",
	"mediated_mailbox_sync",
	"mediated_mailbox_ui",
}

const uiRole = "mediated_mailbox_ui"

// The rows every test below starts from, written by the superuser, whom no policy applies to.
const (
	accountA  = "acct-a"
	accountB  = "acct-b"
	planA     = "00000000-0000-0000-0000-00000000000a"
	candidate = "example.com"
)

// seeded opens the transaction a test works in and writes two accounts, a plan and a candidate for the
// first, an operation-log entry for its plan, and an audit row.
func seeded(t *testing.T) pgx.Tx {
	t.Helper()
	ctx := t.Context()
	tx, err := connect(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		"INSERT INTO accounts (account_id, provider) VALUES ('" + accountA + "', 'gmail'), ('" + accountB + "', 'gmail')",
		"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES ('" + planA + "', '" + accountA + "', 'DRAFT', '{}')",
		"INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after) VALUES ('" + planA + "', 'm-1', '{}', '{Receipts}')",
		"INSERT INTO policy_candidates (account_id, domain, signals, score) VALUES ('" + accountA + "', '" + candidate + "', '[]', 0.9)",
		"INSERT INTO audit_log (account_id, actor, action) VALUES ('" + accountA + "', 'agent', 'READ_BODY')",
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	return tx
}

// asRole runs fn as role in a savepoint of tx, with the account set as db/tx sets it, and rolls the
// savepoint back, which also undoes the role.
func asRole(ctx context.Context, tx pgx.Tx, role, account string, fn func(pgx.Tx) error) (err error) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := sp.Rollback(context.Background()); err == nil {
			err = rollbackErr
		}
	}()
	if _, err := sp.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
		return err
	}
	if _, err := sp.Exec(ctx, "SELECT set_config('app.account', $1, true)", account); err != nil {
		return err
	}
	return fn(sp)
}

// as runs one statement through asRole and returns the rows it affected.
func as(ctx context.Context, tx pgx.Tx, role, account, sql string, args ...any) (affected int64, err error) {
	err = asRole(ctx, tx, role, account, func(sp pgx.Tx) error {
		tag, err := sp.Exec(ctx, sql, args...)
		affected = tag.RowsAffected()
		return err
	})
	return affected, err
}

// refusedByGrant reports whether err is PostgreSQL refusing a privilege the role does not hold,
// either a grant or the ownership that DDL needs. A policy's refusal shares the error code and is not
// one.
func refusedByGrant(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" &&
		(strings.HasPrefix(pgErr.Message, "permission denied") || strings.HasPrefix(pgErr.Message, "must be owner"))
}

// refusedByPolicy reports whether err is a row-level security policy refusing a written row.
func refusedByPolicy(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" && strings.HasPrefix(pgErr.Message, "new row violates row-level security policy")
}

// TestTheBootstrapCreatesOneRuntimeRolePerDeployable requires the bootstrap's roles to be exactly the
// migration role and ADR-0075's runtime roles, so the tests below, which run under each runtime role
// in turn, cannot miss one. A runtime role must also hold nothing beyond its grants. It must not be a
// superuser, bypass row-level security, create roles or databases, or belong to another role, since a
// member acts with that role's privileges.
func TestTheBootstrapCreatesOneRuntimeRolePerDeployable(t *testing.T) {
	ctx := t.Context()
	conn := connect(t)
	rows, err := conn.Query(ctx, `
		SELECT r.rolname, r.rolcanlogin, r.rolsuper OR r.rolbypassrls OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication,
			EXISTS (SELECT FROM pg_auth_members m WHERE m.member = r.oid)
		FROM pg_roles r WHERE r.rolname LIKE 'mediated\_mailbox\_%' ORDER BY r.rolname`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		var login, privileged, member bool
		if err := rows.Scan(&name, &login, &privileged, &member); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
		if name == migrationRole {
			continue
		}
		if !login || privileged || member {
			t.Errorf("role %s: can log in %t, holds a role attribute beyond login %t, belongs to another role %t. Want only login", name, login, privileged, member)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := slices.Sorted(slices.Values(append(slices.Clone(runtimeRoles), migrationRole))); !slices.Equal(names, want) {
		t.Errorf("the bootstrap's roles are %v, want %v", names, want)
	}
}

// TestRuntimeRolesCannotChangeTheSchema attempts schema changes under each runtime role. Only the
// migration role owns the schema and the tables (ADR-0048), so each must be refused for want of a
// privilege.
func TestRuntimeRolesCannotChangeTheSchema(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	changes := map[string]string{
		"create a table":       "CREATE TABLE schema_probe (id int)",
		"add a body column":    "ALTER TABLE messages ADD COLUMN body text",
		"drop a table":         "DROP TABLE audit_log",
		"create an index":      "CREATE INDEX ON messages (subject)",
		"drop a policy":        "DROP POLICY reorg_op_log_plan ON reorg_op_log",
		"disable a policy":     "ALTER TABLE messages DISABLE ROW LEVEL SECURITY",
		"create a schema":      "CREATE SCHEMA schema_probe",
		"create a function":    "CREATE FUNCTION schema_probe() RETURNS int LANGUAGE sql AS 'SELECT 1'",
		"change a table owner": "ALTER TABLE audit_log OWNER TO " + uiRole,
	}
	for _, role := range runtimeRoles {
		for _, name := range slices.Sorted(maps.Keys(changes)) {
			t.Run(role+"/"+name, func(t *testing.T) {
				if _, err := as(ctx, tx, role, accountA, changes[name]); !refusedByGrant(err) {
					t.Errorf("%s: got %v, want a refusal for want of a privilege", changes[name], err)
				}
			})
		}
	}
}

// TestNoRuntimeRoleCanAlterTheAuditLog attempts to update, delete and truncate the audit log under
// each runtime role in turn, with the account set to the audit row's own, so a policy cannot be what
// stops the statement. The statements read no column, so an UPDATE or DELETE grant alone would let
// them through. Each must be refused for want of a grant (ADR-0016).
func TestNoRuntimeRoleCanAlterTheAuditLog(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	attempts := map[string]string{
		"update":   "UPDATE audit_log SET actor = 'erased'",
		"delete":   "DELETE FROM audit_log",
		"truncate": "TRUNCATE audit_log",
	}
	for _, role := range runtimeRoles {
		for _, name := range slices.Sorted(maps.Keys(attempts)) {
			t.Run(role+"/"+name, func(t *testing.T) {
				if _, err := as(ctx, tx, role, accountA, attempts[name]); !refusedByGrant(err) {
					t.Errorf("%s: got %v, want a refusal for want of a grant", attempts[name], err)
				}
			})
		}
	}
}

// uiUpdates and uiInserts are the UI's whole write grant (ADR-0021). The test states them rather than
// reading them from the chain, because they are what it checks the chain against.
var (
	uiUpdates = map[string][]string{
		"reorg_plans":       {"status", "approved_at", "approved_by"},
		"policy_candidates": {"status", "reviewed_at", "reviewed_by"},
	}
	uiInserts = []string{"policy_rules"}
)

// TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules attempts every write under the UI's role, an
// insert into every table, an update of every column, a delete from every table and a truncation of
// every table, reading the tables and columns from the database so a table or column added later is
// covered. Each write outside the grant must be refused for want of a grant. The two verbs' own
// writes must succeed, so the refusals are the grant's doing rather than a role that can write
// nothing.
func TestTheUIWritesOnlyItsDecisionColumnsAndPolicyRules(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	columns := map[string][]string{}
	// An identity column generated always can be set only to its default, and PostgreSQL refuses any
	// other value before it checks the grant, so its update sets the default.
	generated := map[string]bool{}
	rows, err := tx.Query(ctx, `
		SELECT c.table_name, c.column_name, c.identity_generation IS NOT DISTINCT FROM 'ALWAYS'
		FROM information_schema.columns c
		JOIN pg_tables p ON p.schemaname = c.table_schema AND p.tablename = c.table_name
		WHERE c.table_schema = 'public' ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var table, column string
		var always bool
		if err := rows.Scan(&table, &column, &always); err != nil {
			t.Fatal(err)
		}
		columns[table] = append(columns[table], column)
		generated[table+"."+column] = always
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for table := range uiUpdates {
		if _, ok := columns[table]; !ok {
			t.Fatalf("the schema has no table %s", table)
		}
	}

	t.Run("approve a plan", func(t *testing.T) {
		n, err := as(ctx, tx, uiRole, accountA, "UPDATE reorg_plans SET status = 'APPROVED', approved_at = now(), approved_by = 'operator' WHERE plan_id = $1", planA)
		if err != nil || n != 1 {
			t.Errorf("approving the plan updated %d rows with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("confirm a candidate", func(t *testing.T) {
		n, err := as(ctx, tx, uiRole, accountA, "UPDATE policy_candidates SET status = 'confirmed', reviewed_at = now(), reviewed_by = 'operator' WHERE account_id = $1 AND domain = $2", accountA, candidate)
		if err != nil || n != 1 {
			t.Errorf("confirming the candidate updated %d rows with error %v, want 1 and no error", n, err)
		}
		n, err = as(ctx, tx, uiRole, accountA, "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES ($1, $2, 'restricted', $3, 'candidate', 'operator')",
			accountA, "candidate."+accountA+"."+candidate, []string{candidate})
		if err != nil || n != 1 {
			t.Errorf("inserting the confirmed rule inserted %d rows with error %v, want 1 and no error", n, err)
		}
	})

	for _, table := range slices.Sorted(maps.Keys(columns)) {
		quoted := pgx.Identifier{table}.Sanitize()
		writes := map[string]string{
			"delete":   "DELETE FROM " + quoted,
			"truncate": "TRUNCATE " + quoted,
		}
		if !slices.Contains(uiInserts, table) {
			writes["insert"] = "INSERT INTO " + quoted + " DEFAULT VALUES"
		}
		for _, column := range columns[table] {
			if !slices.Contains(uiUpdates[table], column) {
				c := pgx.Identifier{column}.Sanitize()
				value := c
				if generated[table+"."+column] {
					value = "DEFAULT"
				}
				writes["update "+column] = "UPDATE " + quoted + " SET " + c + " = " + value
			}
		}
		for _, name := range slices.Sorted(maps.Keys(writes)) {
			t.Run(table+"/"+name, func(t *testing.T) {
				if _, err := as(ctx, tx, uiRole, accountA, writes[name]); !refusedByGrant(err) {
					t.Errorf("%s: got %v, want a refusal for want of a grant", writes[name], err)
				}
			})
		}
	}
}

// opLogWriter stands in for a role whose statements write the operation log. No runtime role writes
// it yet, and the policy applies to every role alike. It holds the plan columns the policy looks up
// (ADR-0075).
const opLogWriter = "check_op_log_writer"

// TestTheOperationLogIsScopedThroughItsPlan reads and writes the operation log under an account that
// does not own the entry's plan, and again under the account that does. The log has no account column,
// so only its policy, which reaches the plan's account, can tell the two apart (ADR-0016). The reads
// run as the UI's role, which reads the log. A read the policy refuses returns no row, and a write
// raises.
func TestTheOperationLogIsScopedThroughItsPlan(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	for _, sql := range []string{
		"CREATE ROLE " + opLogWriter + " NOLOGIN",
		"GRANT SELECT, INSERT, UPDATE ON reorg_op_log TO " + opLogWriter,
		"GRANT USAGE ON SEQUENCE reorg_op_log_seq_seq TO " + opLogWriter,
		"GRANT SELECT (plan_id, account_id) ON reorg_plans TO " + opLogWriter,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	read := func(account string) (n int, err error) {
		err = asRole(ctx, tx, uiRole, account, func(sp pgx.Tx) error {
			return sp.QueryRow(ctx, "SELECT count(*) FROM reorg_op_log WHERE plan_id = $1", planA).Scan(&n)
		})
		return n, err
	}
	const insert = "INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after) VALUES ($1, 'm-2', '{}', '{Receipts}')"
	const update = "UPDATE reorg_op_log SET labels_after = '{Archive}' WHERE plan_id = $1"

	t.Run("read by another account", func(t *testing.T) {
		if n, err := read(accountB); err != nil || n != 0 {
			t.Errorf("read %d entries with error %v, want none and no error", n, err)
		}
	})
	t.Run("read by the plan's account", func(t *testing.T) {
		if n, err := read(accountA); err != nil || n != 1 {
			t.Errorf("read %d entries with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("insert by another account", func(t *testing.T) {
		if _, err := as(ctx, tx, opLogWriter, accountB, insert, planA); !refusedByPolicy(err) {
			t.Errorf("got %v, want the policy to refuse the row", err)
		}
	})
	t.Run("insert by the plan's account", func(t *testing.T) {
		if n, err := as(ctx, tx, opLogWriter, accountA, insert, planA); err != nil || n != 1 {
			t.Errorf("inserted %d rows with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("update by another account", func(t *testing.T) {
		if n, err := as(ctx, tx, opLogWriter, accountB, update, planA); err != nil || n != 0 {
			t.Errorf("updated %d rows with error %v, want none and no error", n, err)
		}
	})
	t.Run("update by the plan's account", func(t *testing.T) {
		if n, err := as(ctx, tx, opLogWriter, accountA, update, planA); err != nil || n != 1 {
			t.Errorf("updated %d rows with error %v, want 1 and no error", n, err)
		}
	})
}
