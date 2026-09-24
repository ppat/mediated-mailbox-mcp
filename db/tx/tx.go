// Package tx is the shared transaction helper. Every unit of data access runs in a transaction that
// set and verified the account through it (ADR-0047).
//
// The account reaches row-level security through the transaction-local setting app.account, which
// every policy in the migration chain reads (ADR-0016). Once a connection has set it, PostgreSQL
// resets it to the empty string when the transaction ends, so a later transaction that never set it
// sees no rows and raises nothing, depending on which connection a pool handed out. No policy can
// raise instead, because no code runs inside the database (ADR-0060). Run therefore reads the setting
// back and fails the transaction before any data access when it does not hold the account.
package tx

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Beginner opens a transaction. A pool and a connection are each one. A transaction also has Begin,
// and Run refuses it, see ErrInsideTransaction.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrInsideTransaction is returned when db is itself a transaction. Begin would open a savepoint,
// and a transaction-local setting survives the savepoint's release, so the outer transaction would go
// on under this unit's account after Run returned. One unit of data access is one transaction for
// one account.
var ErrInsideTransaction = errors.New("a unit of data access opens its own transaction, and db is already one")

// ErrAccountNotSet is returned when the account setting does not read back as the account, so the
// statements would see no rows rather than fail.
var ErrAccountNotSet = errors.New("the transaction's account setting does not hold the account")

// Run runs fn in a transaction on db that has set the account and read it back. It commits when fn
// returns nil and rolls back otherwise. An empty account fails, since it is the state a transaction
// that never set the account is in.
func Run(ctx context.Context, db Beginner, account string, fn func(pgx.Tx) error) error {
	if _, nested := db.(pgx.Tx); nested {
		return ErrInsideTransaction
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// set_config with its third argument true is SET LOCAL, taking the value as a parameter.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.account', $1, true)", account); err != nil {
			return fmt.Errorf("setting the account: %w", err)
		}
		var got string
		if err := tx.QueryRow(ctx, "SELECT coalesce(current_setting('app.account', true), '')").Scan(&got); err != nil {
			return fmt.Errorf("reading the account back: %w", err)
		}
		if got == "" || got != account {
			return fmt.Errorf("%w: it reads back as %q", ErrAccountNotSet, got)
		}
		return fn(tx)
	})
}
