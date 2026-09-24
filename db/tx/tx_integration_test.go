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
// holding a plan for each of two accounts and acting as the UI's role, which reorg_plans' policy
// applies to. The superuser the test connects as would bypass it.
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
