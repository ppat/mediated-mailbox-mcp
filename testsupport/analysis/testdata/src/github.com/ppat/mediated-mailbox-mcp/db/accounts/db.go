// Package accounts stands for a generated data-access subsection.
package accounts

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// DBTX is any database handle.
type DBTX interface{}

// Queries holds the statements.
type Queries struct{ db DBTX }

// New binds the statements to a handle.
func New(db DBTX) *Queries { return &Queries{db: db} }

// WithTx rebinds the statements to a transaction.
func (q *Queries) WithTx(tx pgx.Tx) *Queries { return &Queries{db: tx} }

// Accounts is a statement.
func (q *Queries) Accounts(ctx context.Context) error { return nil }

// Other is a statement.
func (q *Queries) Other(ctx context.Context) error { return nil }
