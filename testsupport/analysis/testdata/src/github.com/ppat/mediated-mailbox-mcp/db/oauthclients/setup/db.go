// Package setup stands for a generated data-access subsection.
package setup

import "context"

// DBTX is any database handle.
type DBTX interface{}

// Queries holds the statements.
type Queries struct{ db DBTX }

// New binds the statements to a handle.
func New(db DBTX) *Queries { return &Queries{db: db} }

// SetupClients is a statement.
func (q *Queries) SetupClients(ctx context.Context) error { return nil }

// AddClient is a statement.
func (q *Queries) AddClient(ctx context.Context) error { return nil }

// ReplaceClient is a statement.
func (q *Queries) ReplaceClient(ctx context.Context) error { return nil }

// RemoveClient is a statement.
func (q *Queries) RemoveClient(ctx context.Context) error { return nil }

// ClientIdentity is a statement.
func (q *Queries) ClientIdentity(ctx context.Context) error { return nil }
