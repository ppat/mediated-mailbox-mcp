// Package secret stands for a generated data-access subsection.
package secret

import "context"

// DBTX is any database handle.
type DBTX interface{}

// Queries holds the statements.
type Queries struct{ db DBTX }

// New binds the statements to a handle.
func New(db DBTX) *Queries { return &Queries{db: db} }

// ReplaceSealedClient is a statement.
func (q *Queries) ReplaceSealedClient(ctx context.Context) error { return nil }

// Other is a statement.
func (q *Queries) Other(ctx context.Context) error { return nil }
