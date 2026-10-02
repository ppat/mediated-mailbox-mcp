// Package pgx stands for the database driver, its transaction type as the txhelper cases need it,
// and its connection, batch and table identifier as the rawsql cases need them.
package pgx

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
)

// Tx is a transaction.
type Tx interface{ Commit() error }

// Rows are a query's rows.
type Rows interface{ Close() }

// Row is a query's one row.
type Row interface{ Scan(dest ...any) error }

// Batch is a batch of statements.
type Batch struct{}

// Identifier names a table.
type Identifier []string

// Conn is a connection.
type Conn struct{}

// Query runs a statement and reads its rows.
func (*Conn) Query(ctx context.Context, sql string, args ...any) (Rows, error) { return nil, nil }

// QueryRow runs a statement and reads its one row.
func (*Conn) QueryRow(ctx context.Context, sql string, args ...any) Row { return nil }

// Exec runs a statement.
func (*Conn) Exec(ctx context.Context, sql string, args ...any) error { return nil }

// SendBatch runs a batch of statements.
func (*Conn) SendBatch(ctx context.Context, b *Batch) error { return nil }

// CopyFrom copies rows into a table.
func (*Conn) CopyFrom(ctx context.Context, table Identifier, columns []string, rows [][]any) (int64, error) {
	return 0, nil
}

// Prepare prepares a statement under a name.
func (*Conn) Prepare(ctx context.Context, name, sql string) error { return nil }

// PgConn is the connection's lower-level connection.
func (*Conn) PgConn() *pgconn.PgConn { return nil }
