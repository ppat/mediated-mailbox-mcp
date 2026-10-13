// Package tx is the shared transaction helper. Every unit of data access runs in a transaction that
// set and verified the account through it (ADR-0047).
//
// The account reaches every statement through the transaction-local setting app.account, which each
// statement compares its tables' account column with, current_setting('app.account') (ADR-0135). The
// account enters nowhere else, so no statement can be given an account other than its transaction's.
// Once a connection has set the setting, PostgreSQL resets it to the empty string when the transaction
// ends, so a later transaction that never set it matches no rows and raises nothing, depending on
// which connection a pool handed out. Nothing in the database can raise instead, because no code runs
// there (ADR-0060). Run therefore reads the setting back and fails the transaction before any data
// access when it does not hold the account.
//
// The base policy belongs to no account, and RunBase is the one unit of data access that names none
// (ADR-0112). It sets the account to the empty string, transaction-local, and reads it back, so a
// statement comparing the transaction's account matches no account's rows. The txhelper analyser holds
// RunBase to the base policy's own statements, in db/policyrules/base, and those statements to RunBase.
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

// BeginnerWithOptions opens a transaction with options. A pool and a connection are each one, and a
// transaction is not, since pgx.Tx has no BeginTx.
type BeginnerWithOptions interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Snapshot returns db as a Beginner whose every transaction is repeatable read and read-only. Every
// statement a Run over it makes reads the one snapshot its first statement took, and none can write.
// A unit whose statements must agree on the state they read, such as a read whose classification
// feeds the statements after it, runs over it. Run's default is the database's, read committed, where
// each statement reads what was committed when it starts. db is never a transaction, so the wrapper
// cannot hide one from Run's refusal of a nested transaction.
func Snapshot(db BeginnerWithOptions) Beginner { return snapshot{db: db} }

type snapshot struct{ db BeginnerWithOptions }

func (s snapshot) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
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
		if _, err := tx.Exec(ctx, "-- name: SetTransactionAccount :exec\nSELECT set_config('app.account', $1, true)", account); err != nil {
			return fmt.Errorf("setting the account: %w", err)
		}
		var got string
		if err := tx.QueryRow(ctx, "-- name: ReadTransactionAccount :one\nSELECT coalesce(current_setting('app.account', true), '')").Scan(&got); err != nil {
			return fmt.Errorf("reading the account back: %w", err)
		}
		if got == "" || got != account {
			return fmt.Errorf("%w: it reads back as %q", ErrAccountNotSet, got)
		}
		return fn(tx)
	})
}

// ErrBaseNotSet is returned when a base-policy transaction's account does not read back as empty.
var ErrBaseNotSet = errors.New("the transaction's account setting does not read back as empty")

// RunBase runs fn in a base-policy transaction on db, one whose account is the empty string, set
// transaction-local and read back (ADR-0112). It commits when fn returns nil and rolls back otherwise.
// Only the base policy's statements run in it, which the txhelper analyser enforces.
func RunBase(ctx context.Context, db Beginner, fn func(pgx.Tx) error) error {
	if _, nested := db.(pgx.Tx); nested {
		return ErrInsideTransaction
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "-- name: SetBaseScope :exec\nSELECT set_config('app.account', '', true)"); err != nil {
			return fmt.Errorf("setting the base policy's scope: %w", err)
		}
		var account string
		if err := tx.QueryRow(ctx, "-- name: ReadBaseScope :one\nSELECT coalesce(current_setting('app.account', true), '')").Scan(&account); err != nil {
			return fmt.Errorf("reading the base policy's scope back: %w", err)
		}
		if account != "" {
			return fmt.Errorf("%w: it reads back as %q", ErrBaseNotSet, account)
		}
		return fn(tx)
	})
}
