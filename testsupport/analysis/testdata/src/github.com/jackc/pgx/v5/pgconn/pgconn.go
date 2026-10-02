// Package pgconn stands for the database driver's lower-level connection, as the rawsql cases need it.
package pgconn

import (
	"context"
	"io"
)

// Batch is a batch of statements at the protocol level.
type Batch struct{}

// Pipeline sends statements without waiting for their results.
type Pipeline struct{}

// SendQueryParams queues a statement on the pipeline.
func (*Pipeline) SendQueryParams(sql string) {}

// PgConn is a lower-level connection.
type PgConn struct{}

// Exec runs statements by the simple protocol.
func (*PgConn) Exec(ctx context.Context, sql string) error { return nil }

// ExecParams runs a statement by the extended protocol.
func (*PgConn) ExecParams(ctx context.Context, sql string, values [][]byte) error { return nil }

// Prepare prepares a statement.
func (*PgConn) Prepare(ctx context.Context, name, sql string, oids []uint32) error { return nil }

// ExecPrepared runs a prepared statement by its name.
func (*PgConn) ExecPrepared(ctx context.Context, name string, values [][]byte) error { return nil }

// CopyTo copies a statement's rows out.
func (*PgConn) CopyTo(ctx context.Context, w io.Writer, sql string) error { return nil }

// CopyFrom copies rows in by a statement.
func (*PgConn) CopyFrom(ctx context.Context, r io.Reader, sql string) error { return nil }

// ExecBatch runs a batch.
func (*PgConn) ExecBatch(ctx context.Context, b *Batch) error { return nil }

// StartPipeline starts a pipeline.
func (*PgConn) StartPipeline(ctx context.Context) *Pipeline { return nil }

// Deallocate releases a prepared statement by its name.
func (*PgConn) Deallocate(ctx context.Context, name string) error { return nil }

// EscapeString escapes a string literal.
func (*PgConn) EscapeString(s string) (string, error) { return s, nil }
