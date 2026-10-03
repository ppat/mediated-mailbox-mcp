//go:build banproof

package api

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// This file crosses the two scopes of the policy on purpose, once each way the txhelper analyser
// refuses, and banproof requires the wants below from go vet. The base policy's statements run only in
// a base-policy transaction, and no other subsection's do, so the empty account a base-policy
// transaction sets is never read where an account's rows were meant (ADR-0112).
func crossedScopes(ctx context.Context, db tx.Beginner, account string) error {
	if err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		_, err := base.New(t).BaseRules(ctx) // want vetcheck "calls base.New in a transaction tx.Run opened for an account"
		return err
	}); err != nil {
		return err
	}
	return tx.RunBase(ctx, db, func(t pgx.Tx) error {
		_, err := manage.New(t).ComposedRules(ctx, account) // want vetcheck "calls manage.New in a base-policy transaction"
		return err
	})
}
