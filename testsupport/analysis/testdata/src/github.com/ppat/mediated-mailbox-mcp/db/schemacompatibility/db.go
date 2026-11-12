// Package schemacompatibility stands for a generated data-access subsection.
package schemacompatibility

import "context"

// DBTX is any database handle.
type DBTX interface{}

// Queries holds the statements.
type Queries struct{ db DBTX }

// New binds the statements to a handle.
func New(db DBTX) *Queries { return &Queries{db: db} }

// SchemaVersions is a statement.
func (q *Queries) SchemaVersions(ctx context.Context) error { return nil }

// Other is a statement.
func (q *Queries) Other(ctx context.Context) error { return nil }
