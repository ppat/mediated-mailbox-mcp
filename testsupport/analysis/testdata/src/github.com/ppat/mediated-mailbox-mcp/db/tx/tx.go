// Package tx stands for the transaction helper.
package tx

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Beginner opens a transaction.
type Beginner interface{}

// Run runs fn in a transaction that set the account.
func Run(ctx context.Context, db Beginner, account string, fn func(pgx.Tx) error) error { return nil }

// RunBase runs fn in a base-policy transaction, which names no account.
func RunBase(ctx context.Context, db Beginner, fn func(pgx.Tx) error) error { return nil }
