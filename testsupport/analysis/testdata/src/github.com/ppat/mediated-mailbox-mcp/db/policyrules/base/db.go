// Package base stands for the base policy's generated data-access subsection, whose statements run
// only in a base-policy transaction.
package base

import "context"

// DBTX is any database handle.
type DBTX interface{}

// Queries holds the statements.
type Queries struct{ db DBTX }

// New binds the statements to a handle.
func New(db DBTX) *Queries { return &Queries{db: db} }

// BaseRules is a statement.
func (q *Queries) BaseRules(ctx context.Context) error { return nil }
