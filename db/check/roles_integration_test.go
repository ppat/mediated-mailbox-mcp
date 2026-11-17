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

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// The tests below run statements as the application user, the one user the API server, the UI and
// the worker connect as (ADR-0143), against the migration chain pgrun applied from empty to this
// package's database with its grants made to that user. Each works inside one transaction it rolls
// back, and runs each statement in a savepoint of its own as the user, so a refused statement leaves
// the rest of the test able to run and nothing reaches the other tests of this package.

// appUser is the application user.
const appUser = postgres.ApplicationUser

// The rows every test below starts from, written by the superuser the tests connect as.
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
		"INSERT INTO audit_log (account_id, actor, action, stage, reason, sender_class, scan_state) VALUES ('" + accountA + "', 'agent', 'READ_BODY', 'serve', 'released', 'normal', 'scanned')",
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
// either a grant or the ownership that DDL needs.
func refusedByGrant(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" &&
		(strings.HasPrefix(pgErr.Message, "permission denied") || strings.HasPrefix(pgErr.Message, "must be owner"))
}

// TestTheBootstrapCreatesTheTwoUsers requires the bootstrap's users to be exactly the migrator's user
// and the application user, so the tests below, which run as the application user, run as every user a
// program connects as (ADR-0143). The application user must also hold nothing beyond its grants, so no
// role attribute but LOGIN. It must not be a superuser, bypass row-level security, create roles or
// databases or replicate, each a power no grant gives or takes away, and it must not belong to another
// role, since a member acts with that role's privileges.
func TestTheBootstrapCreatesTheTwoUsers(t *testing.T) {
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
		if name == postgres.MigrationRole {
			continue
		}
		if !login || privileged || member {
			t.Errorf("user %s: can log in %t, holds a role attribute beyond login %t, belongs to another role %t. Want only login", name, login, privileged, member)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{appUser, postgres.MigrationRole}; !slices.Equal(names, want) {
		t.Errorf("the bootstrap's users are %v, want %v", names, want)
	}
}

// TestTheApplicationUserCannotChangeTheSchema attempts schema changes as the application user. Only
// the migrator's user owns the schema and the tables (ADR-0048), so each must be refused for want of a
// privilege.
func TestTheApplicationUserCannotChangeTheSchema(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	changes := map[string]string{
		"create a table":       "CREATE TABLE schema_probe (id int)",
		"add a body column":    "ALTER TABLE messages ADD COLUMN body text",
		"drop a table":         "DROP TABLE audit_log",
		"create an index":      "CREATE INDEX ON messages (subject)",
		"create a policy":      "CREATE POLICY schema_probe ON messages USING (true)",
		"enable row security":  "ALTER TABLE messages ENABLE ROW LEVEL SECURITY",
		"create a schema":      "CREATE SCHEMA schema_probe",
		"create a function":    "CREATE FUNCTION schema_probe() RETURNS int LANGUAGE sql AS 'SELECT 1'",
		"change a table owner": "ALTER TABLE audit_log OWNER TO " + appUser,
	}
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		t.Run(name, func(t *testing.T) {
			if _, err := as(ctx, tx, appUser, accountA, changes[name]); !refusedByGrant(err) {
				t.Errorf("%s: got %v, want a refusal for want of a privilege", changes[name], err)
			}
		})
	}
}

// TestTheApplicationUserCannotAlterTheAuditLog attempts to update, delete and truncate the audit log as
// the application user, with the account set to the audit row's own. The statements read no column,
// so an UPDATE or DELETE grant alone would let them through. Each must be refused for want of a grant
// (ADR-0016). The user's own insert of an audit row succeeds, so the refusals are the grant's doing.
func TestTheApplicationUserCannotAlterTheAuditLog(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if n, err := as(ctx, tx, appUser, accountA, "INSERT INTO audit_log (account_id, actor, action, stage, reason, sender_class, scan_state) VALUES (current_setting('app.account'), 'agent', 'READ_BODY', 'serve', 'released', 'normal', 'scanned')"); err != nil || n != 1 {
		t.Fatalf("recording an audit row inserted %d rows with error %v, want 1 and no error", n, err)
	}
	attempts := map[string]string{
		"update":   "UPDATE audit_log SET actor = 'erased'",
		"delete":   "DELETE FROM audit_log",
		"truncate": "TRUNCATE audit_log",
	}
	for _, name := range slices.Sorted(maps.Keys(attempts)) {
		t.Run(name, func(t *testing.T) {
			if _, err := as(ctx, tx, appUser, accountA, attempts[name]); !refusedByGrant(err) {
				t.Errorf("%s: got %v, want a refusal for want of a grant", attempts[name], err)
			}
		})
	}
}

// TestTheApplicationUserCannotAlterThePolicyHistory attempts to update, delete and truncate the policy
// history as the application user, with the account set to the history row's own. Each must be refused
// for want of a grant, so who lifted a restriction survives a compromise of the process that wrote it
// (ADR-0102). The user's own append of a history row succeeds, so the refusals are the grant's doing.
func TestTheApplicationUserCannotAlterThePolicyHistory(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	if n, err := as(ctx, tx, appUser, accountA, "INSERT INTO policy_changes (account_id, actor, action, rule_id, suffixes_after) VALUES (current_setting('app.account'), 'operator', 'added', 'operator.example.com', '{example.com}')"); err != nil || n != 1 {
		t.Fatalf("appending a history row inserted %d rows with error %v, want 1 and no error", n, err)
	}
	attempts := map[string]string{
		"update":   "UPDATE policy_changes SET actor = 'erased'",
		"delete":   "DELETE FROM policy_changes",
		"truncate": "TRUNCATE policy_changes",
	}
	for _, name := range slices.Sorted(maps.Keys(attempts)) {
		t.Run(name, func(t *testing.T) {
			if _, err := as(ctx, tx, appUser, accountA, attempts[name]); !refusedByGrant(err) {
				t.Errorf("%s: got %v, want a refusal for want of a grant", attempts[name], err)
			}
		})
	}
}

// opLogWriter stands in for a user whose statements write the operation log. No statement writes it
// yet, so the application user holds no grant to. It holds the plan columns the join to a plan reads (ADR-0135).
const opLogWriter = "check_op_log_writer"

// TestTheOperationLogIsReachedThroughItsPlan reads and appends to the operation log in the form the
// statement check requires of it, through a join to its plan tied to the transaction's account, under
// an account that does not own the entry's plan and again under the account that does. The log has no
// account column, so only the plan can tell the two apart (ADR-0135). The reads run as the application
// user, which reads the log. Under the other account the read finds no entry and the append writes none.
func TestTheOperationLogIsReachedThroughItsPlan(t *testing.T) {
	ctx := t.Context()
	tx := seeded(t)
	for _, sql := range []string{
		"CREATE ROLE " + opLogWriter + " NOLOGIN",
		"GRANT SELECT, INSERT ON reorg_op_log TO " + opLogWriter,
		"GRANT USAGE ON SEQUENCE reorg_op_log_seq_seq TO " + opLogWriter,
		"GRANT SELECT (plan_id, account_id) ON reorg_plans TO " + opLogWriter,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	const read = `SELECT count(*) FROM reorg_op_log AS l INNER JOIN reorg_plans AS p ON l.plan_id = p.plan_id
		WHERE p.account_id = current_setting('app.account') AND l.plan_id = $1`
	const appendOp = `INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after)
		SELECT p.plan_id, 'm-2', '{}', '{Receipts}' FROM reorg_plans AS p
		WHERE p.account_id = current_setting('app.account') AND p.plan_id = $1`
	count := func(account string) (n int, err error) {
		err = asRole(ctx, tx, appUser, account, func(sp pgx.Tx) error { return sp.QueryRow(ctx, read, planA).Scan(&n) })
		return n, err
	}

	t.Run("read by another account", func(t *testing.T) {
		if n, err := count(accountB); err != nil || n != 0 {
			t.Errorf("read %d entries with error %v, want none and no error", n, err)
		}
	})
	t.Run("read by the plan's account", func(t *testing.T) {
		if n, err := count(accountA); err != nil || n != 1 {
			t.Errorf("read %d entries with error %v, want 1 and no error", n, err)
		}
	})
	t.Run("append by another account", func(t *testing.T) {
		if n, err := as(ctx, tx, opLogWriter, accountB, appendOp, planA); err != nil || n != 0 {
			t.Errorf("appended %d entries with error %v, want none and no error", n, err)
		}
	})
	t.Run("append by the plan's account", func(t *testing.T) {
		if n, err := as(ctx, tx, opLogWriter, accountA, appendOp, planA); err != nil || n != 1 {
			t.Errorf("appended %d entries with error %v, want 1 and no error", n, err)
		}
	})
}
