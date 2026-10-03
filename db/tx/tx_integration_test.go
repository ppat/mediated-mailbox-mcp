//go:build integration

package tx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// countPlans is the data access the tests run, a read of reorg_plans, whose policy scopes it to the
// transaction's account.
const countPlans = "SELECT count(*) FROM reorg_plans WHERE account_id = $1"

// uiConnection returns one connection, so every transaction below runs on the same server session,
// holding a plan for each of two accounts, a base rule and one of the first account's own rules, and
// acting as the UI's role, which reorg_plans' and policy_rules' policies apply to. The superuser the test connects as would bypass it.
func uiConnection(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx := t.Context()
	conn, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	setup := []string{
		"INSERT INTO accounts (account_id, provider) VALUES ('acct-a', 'gmail'), ('acct-b', 'gmail') ON CONFLICT DO NOTHING",
		`INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES
			('00000000-0000-0000-0000-00000000000a', 'acct-a', 'DRAFT', '{}'),
			('00000000-0000-0000-0000-00000000000b', 'acct-b', 'DRAFT', '{}')
			ON CONFLICT DO NOTHING`,
		`INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES
			(NULL, 'operator.example.org', 'restricted', '{example.org}', 'operator', 'operator'),
			('acct-a', 'operator.example.net', 'restricted', '{example.net}', 'operator', 'operator')
			ON CONFLICT DO NOTHING`,
		"SET ROLE mediated_mailbox_ui",
	}
	for _, sql := range setup {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	return conn
}

// TestRunScopesTheTransactionToTheAccount is the helper's positive case, so the failure below is the
// unset account's doing and not a helper that fails every transaction.
func TestRunScopesTheTransactionToTheAccount(t *testing.T) {
	conn := uiConnection(t)
	for _, account := range []string{"acct-a", "acct-b"} {
		var own, other int
		err := tx.Run(t.Context(), conn, account, func(tx pgx.Tx) error {
			if err := tx.QueryRow(t.Context(), countPlans, account).Scan(&own); err != nil {
				return err
			}
			return tx.QueryRow(t.Context(), "SELECT count(*) FROM reorg_plans WHERE account_id <> $1", account).Scan(&other)
		})
		if err != nil {
			t.Fatalf("%s: %v", account, err)
		}
		if own != 1 || other != 0 {
			t.Errorf("%s sees %d of its own plans and %d of the other account's, want 1 and 0", account, own, other)
		}
	}
}

// TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage proves the injection of docs/VERIFICATIONS.md.
// On a connection that set the account in an earlier transaction, a transaction that never sets it
// gets an empty page and no error. That is shown first, without the helper, so the helper's failure
// below is the compensation for it and not a refusal the database would have raised anyway.
func TestAnUnsetAccountFailsRatherThanReturningAnEmptyPage(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	if err := tx.Run(ctx, conn, "acct-a", func(pgx.Tx) error { return nil }); err != nil {
		t.Fatalf("setting the account in an earlier transaction: %v", err)
	}

	var plans int
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, countPlans, "acct-a").Scan(&plans)
	})
	if err != nil || plans != 0 {
		t.Fatalf("without the helper, a transaction that never set the account read %d plans with error %v, want an empty page and no error", plans, err)
	}

	called := false
	err = tx.Run(ctx, conn, "", func(tx pgx.Tx) error {
		called = true
		return tx.QueryRow(ctx, countPlans, "acct-a").Scan(&plans)
	})
	if !errors.Is(err, tx.ErrAccountNotSet) {
		t.Errorf("a transaction that did not set the account returned %v and read %d plans, want the unset account refused", err, plans)
	}
	if called {
		t.Error("the data access ran in a transaction that did not set the account")
	}
}

// TestTheAccountEndsWithItsTransaction requires the setting to be transaction-local. A session-wide
// setting would scope the next transaction on the connection to the previous one's account, however
// that transaction was opened.
func TestTheAccountEndsWithItsTransaction(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	if err := tx.Run(ctx, conn, "acct-a", func(pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var setting string
	if err := conn.QueryRow(ctx, "SELECT coalesce(current_setting('app.account', true), '')").Scan(&setting); err != nil {
		t.Fatal(err)
	}
	if setting != "" {
		t.Errorf("after the transaction ended the connection's account is %q, want it unset", setting)
	}
	var plans int
	if err := conn.QueryRow(ctx, countPlans, "acct-a").Scan(&plans); err != nil || plans != 0 {
		t.Errorf("after the transaction ended a statement read %d plans with error %v, want none", plans, err)
	}
}

// TestRunRefusesATransaction passes an open transaction as the Beginner. Run would open a savepoint
// in it, and the account would outlive the savepoint in the outer transaction, so the outer
// transaction's later statements would run under this unit's account. Run must refuse before setting
// anything.
func TestRunRefusesATransaction(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	outer, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	called := false
	err = tx.Run(ctx, outer, "acct-b", func(pgx.Tx) error {
		called = true
		return nil
	})
	if !errors.Is(err, tx.ErrInsideTransaction) || called {
		t.Errorf("Run inside a transaction returned %v and ran fn %t, want it refused before fn", err, called)
	}
	var setting string
	if err := outer.QueryRow(ctx, "SELECT coalesce(current_setting('app.account', true), '')").Scan(&setting); err != nil {
		t.Fatal(err)
	}
	if setting != "" {
		t.Errorf("after Run returned, the outer transaction's account is %q, want it unset", setting)
	}
}

// editBase is a write of the base policy, which row-level security admits only from the UI's role in a
// base-policy transaction (ADR-0112). It changes the rule to what it already holds, so the tests can
// repeat it.
const editBase = "UPDATE policy_rules SET domain_suffix = domain_suffix WHERE account_id IS NULL AND rule_id = 'operator.example.org'"

// TestRunBaseWritesTheBasePolicyAndReadsNoAccount is the base-policy transaction's positive case. The
// edit lands, and no account's rule is read, while an account's transaction on the same connection reads
// its own rule, so the empty count is the base-policy transaction's doing.
func TestRunBaseWritesTheBasePolicyAndReadsNoAccount(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	const accountRules = "SELECT count(*) FROM policy_rules WHERE account_id IS NOT NULL"
	var edited int64
	var own int
	err := tx.RunBase(ctx, conn, func(x pgx.Tx) error {
		tag, err := x.Exec(ctx, editBase)
		edited = tag.RowsAffected()
		if err != nil {
			return err
		}
		return x.QueryRow(ctx, accountRules).Scan(&own)
	})
	if err != nil || edited != 1 || own != 0 {
		t.Errorf("the base-policy transaction edited %d base rules and read %d account rules with error %v, want 1, 0 and no error", edited, own, err)
	}
	var accountOwn int
	if err := tx.Run(ctx, conn, "acct-a", func(x pgx.Tx) error { return x.QueryRow(ctx, accountRules).Scan(&accountOwn) }); err != nil || accountOwn != 1 {
		t.Errorf("acct-a's transaction read %d of its rules with error %v, want 1 and no error", accountOwn, err)
	}
}

// TestTheBasePolicyEndsWithItsTransaction requires the base policy's setting to be transaction-local,
// as the account is. On a connection that ran a base-policy transaction, a transaction that never set
// the base policy's scope edits no base rule and raises nothing, which is the silent state the helper
// exists to prevent, and an account's transaction edits none either.
func TestTheBasePolicyEndsWithItsTransaction(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	if err := tx.RunBase(ctx, conn, func(pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var setting string
	if err := conn.QueryRow(ctx, "SELECT coalesce(current_setting('app.base', true), '')").Scan(&setting); err != nil {
		t.Fatal(err)
	}
	if setting != "" {
		t.Errorf("after the base-policy transaction ended the connection's app.base is %q, want it unset", setting)
	}
	var plain, account int64
	err := pgx.BeginFunc(ctx, conn, func(x pgx.Tx) error {
		tag, err := x.Exec(ctx, editBase)
		plain = tag.RowsAffected()
		return err
	})
	if err != nil || plain != 0 {
		t.Errorf("a transaction outside the helper edited %d base rules with error %v, want none and no error", plain, err)
	}
	err = tx.Run(ctx, conn, "acct-a", func(x pgx.Tx) error {
		tag, err := x.Exec(ctx, editBase)
		account = tag.RowsAffected()
		return err
	})
	if err != nil || account != 0 {
		t.Errorf("acct-a's transaction edited %d base rules with error %v, want none and no error", account, err)
	}
}

// TestRunBaseRefusesATransaction passes an open transaction to RunBase, which must refuse before
// setting anything, as Run does.
func TestRunBaseRefusesATransaction(t *testing.T) {
	ctx := t.Context()
	conn := uiConnection(t)
	outer, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	called := false
	err = tx.RunBase(ctx, outer, func(pgx.Tx) error {
		called = true
		return nil
	})
	if !errors.Is(err, tx.ErrInsideTransaction) || called {
		t.Errorf("RunBase inside a transaction returned %v and ran fn %t, want it refused before fn", err, called)
	}
	var setting string
	if err := outer.QueryRow(ctx, "SELECT coalesce(current_setting('app.base', true), '')").Scan(&setting); err != nil {
		t.Fatal(err)
	}
	if setting != "" {
		t.Errorf("after RunBase returned, the outer transaction's app.base is %q, want it unset", setting)
	}
}
