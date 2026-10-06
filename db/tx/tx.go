// Package tx is the shared transaction helper. Every unit of data access runs in a transaction that
// set and verified the account through it (ADR-0047).
//
// The account reaches row-level security through the transaction-local setting app.account, which
// every policy in the migration chain reads (ADR-0016). Once a connection has set it, PostgreSQL
// resets it to the empty string when the transaction ends, so a later transaction that never set it
// sees no rows and raises nothing, depending on which connection a pool handed out. No policy can
// raise instead, because no code runs inside the database (ADR-0060). Run therefore reads the setting
// back and fails the transaction before any data access when it does not hold the account.
//
// The base policy belongs to no account, and RunBase is the one unit of data access that names none
// (ADR-0112). It sets the account to the empty string and app.base to on, both transaction-local, and
// reads both back. Row-level security lets the UI's role write the base policy's rows only under those
// two settings, and the empty account reads no account's rows. The txhelper analyser holds RunBase to
// the base policy's own statements, in db/policyrules/base, and those statements to RunBase.
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

// ErrBaseNotSet is returned when a base-policy transaction's settings do not read back as one that
// names no account and the base policy.
var ErrBaseNotSet = errors.New("the transaction's settings do not name the base policy and no account")

// RunBase runs fn in a base-policy transaction on db, one whose account is the empty string and whose
// app.base reads on, both set transaction-local and read back (ADR-0112). It commits when fn returns
// nil and rolls back otherwise. Only the base policy's statements run in it, which the txhelper
// analyser enforces.
func RunBase(ctx context.Context, db Beginner, fn func(pgx.Tx) error) error {
	if _, nested := db.(pgx.Tx); nested {
		return ErrInsideTransaction
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.account', '', true), set_config('app.base', 'on', true)"); err != nil {
			return fmt.Errorf("setting the base policy's scope: %w", err)
		}
		var account, base string
		err := tx.QueryRow(ctx, "SELECT coalesce(current_setting('app.account', true), ''), coalesce(current_setting('app.base', true), '')").Scan(&account, &base)
		if err != nil {
			return fmt.Errorf("reading the base policy's scope back: %w", err)
		}
		if account != "" || base != "on" {
			return fmt.Errorf("%w: the account reads back as %q and app.base as %q", ErrBaseNotSet, account, base)
		}
		return fn(tx)
	})
}
