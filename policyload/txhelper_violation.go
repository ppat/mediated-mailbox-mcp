//go:build banproof

package policyload

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/policyrules"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// This file calls a generated data-access function outside the transaction helper on purpose, once
// each way the txhelper analyser refuses, and banproof requires the wants below from go vet. A
// statement run this way can run in a transaction that did not set the account, where it sees no
// rows and raises nothing.
func outsideTheHelper(ctx context.Context, db tx.Beginner, pool policyrules.DBTX, account string) error {
	if _, err := policyrules.New(pool).PolicyRules(ctx, account); err != nil { // want vetcheck "calls policyrules.New outside a function literal passed to tx.Run"
		return err
	}
	build := policyrules.New // want vetcheck "uses policyrules.New as a value"
	_ = build
	if err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		_ = t
		_, err := policyrules.New(pool).PolicyRules(ctx, account) // want vetcheck "calls policyrules.New on a handle other than the transaction tx.Run hands the enclosing literal"
		return err
	}); err != nil {
		return err
	}
	if err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		rebound := policyrules.New(t)
		_, err := rebound.WithTx(t).PolicyRules(ctx, account) // want vetcheck "rebinds policyrules queries with WithTx"
		return err
	}); err != nil {
		return err
	}
	if err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		return tx.Run(ctx, db, "another", func(inner pgx.Tx) error {
			_ = inner
			_, err := policyrules.New(t).PolicyRules(ctx, account) // want vetcheck "calls policyrules.New on the transaction of an enclosing literal passed to tx.Run, not the innermost one"
			return err
		})
	}); err != nil {
		return err
	}
	if err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		held := &t // want vetcheck "takes the address of the transaction tx.Run hands the literal"
		*held = nil
		t = nil // want vetcheck "assigns to the transaction tx.Run hands the literal"
		_, err := policyrules.New(t).PolicyRules(ctx, account)
		return err
	}); err != nil {
		return err
	}
	return tx.Run(ctx, db, account, readRules) // want vetcheck "passes tx.Run a function that is not a function literal"
}

func readRules(pgx.Tx) error { return nil }
