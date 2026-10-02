// Package store stands for a package of the UI that runs statements by hand.
package store

import (
	"context"
	"io"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ppat/mediated-mailbox-mcp/db/statements"
)

// Database embeds a generated subsection's handle, as the UI's server does.
type Database interface {
	statements.DBTX
	Ping(ctx context.Context) error
}

// wrapper is a handle the UI declares itself.
type wrapper struct{ conn *pgx.Conn }

func (w wrapper) Query(ctx context.Context, sql string) error {
	_, err := w.conn.Query(ctx, sql) // want `The method Query runs a statement the data-access library did not generate`
	return err
}

// Text is an alias of the statement's text.
type Text = string

type aliased interface {
	Exec(ctx context.Context, sql Text) error
}

func driver(ctx context.Context, conn *pgx.Conn) {
	_ = conn.QueryRow(ctx, "SELECT client_secret FROM oauth_clients")    // want `The method QueryRow runs a statement`
	_ = conn.Exec(ctx, "DELETE FROM oauth_clients")                      // want `The method Exec runs a statement`
	_ = conn.SendBatch(ctx, &pgx.Batch{})                                // want `The method SendBatch runs a statement`
	_, _ = conn.CopyFrom(ctx, pgx.Identifier{"oauth_clients"}, nil, nil) // want `The method CopyFrom runs a statement`
	run := conn.Query                                                    // want `The method Query runs a statement`
	_, _ = run(ctx, "SELECT 1")
}

func handles(ctx context.Context, db Database, h statements.DBTX, w wrapper, a aliased) {
	_ = db.QueryRow(ctx, "SELECT client_secret FROM oauth_clients") // want `The method QueryRow runs a statement`
	_ = h.Exec(ctx, "SELECT 1")                                     // want `The method Exec runs a statement`
	_ = w.Query(ctx, "SELECT 1")                                    // want `The method Query runs a statement`
	_ = a.Exec(ctx, "SELECT 1")                                     // want `The method Exec runs a statement`
	_ = db.Ping(ctx)
}

func lowerLevel(ctx context.Context, conn *pgx.Conn, pc *pgconn.PgConn) {
	_ = conn.Prepare(ctx, "secret", "SELECT client_secret FROM oauth_clients") // want `The method Prepare runs a statement`
	_ = pc.Exec(ctx, "SELECT 1")                                               // want `The method Exec runs a statement`
	_ = pc.ExecParams(ctx, "SELECT client_secret FROM oauth_clients", nil)     // want `The method ExecParams runs a statement`
	_ = pc.Prepare(ctx, "secret", "SELECT 1", nil)                             // want `The method Prepare runs a statement`
	_ = pc.CopyTo(ctx, nil, "COPY oauth_clients TO STDOUT")                    // want `The method CopyTo runs a statement`
	_ = pc.CopyFrom(ctx, nil, "COPY oauth_clients FROM STDIN")                 // want `The method CopyFrom runs a statement`
	_ = pc.ExecBatch(ctx, &pgconn.Batch{})                                     // want `The method ExecBatch runs a statement`
	pc.StartPipeline(ctx).SendQueryParams("SELECT 1")                          // want `The method StartPipeline runs a statement`
	_ = pc.ExecPrepared(ctx, "secret", nil)
	_ = pc.Deallocate(ctx, "secret")
	_, _ = pc.EscapeString("x")
}

// protocol is an interface the UI declares over the lower-level connection's statement methods.
type protocol interface {
	ExecParams(ctx context.Context, sql string, values [][]byte) error
	CopyTo(ctx context.Context, w io.Writer, sql string) error
	CopyFrom(ctx context.Context, r io.Reader, sql string) error
	ExecBatch(ctx context.Context, b *pgconn.Batch) error
	StartPipeline(ctx context.Context) *pgconn.Pipeline
}

func declared(ctx context.Context, p protocol) {
	_ = p.ExecParams(ctx, "SELECT client_secret FROM oauth_clients", nil) // want `The method ExecParams runs a statement`
	_ = p.CopyTo(ctx, nil, "COPY oauth_clients TO STDOUT")                // want `The method CopyTo runs a statement`
	_ = p.CopyFrom(ctx, nil, "COPY oauth_clients FROM STDIN")             // want `The method CopyFrom runs a statement`
	_ = p.ExecBatch(ctx, &pgconn.Batch{})                                 // want `The method ExecBatch runs a statement`
	_ = p.StartPipeline(ctx)                                              // want `The method StartPipeline runs a statement`
}

// lowLookalikes share a lower-level method's name and not its parameters or result.
type lowLookalikes interface {
	CopyTo(ctx context.Context, w io.Writer, n int) error
	StartPipeline(ctx context.Context) error
	ExecBatch(ctx context.Context, n int) error
}

func lowLookalike(ctx context.Context, l lowLookalikes) {
	_ = l.CopyTo(ctx, nil, 1)
	_ = l.StartPipeline(ctx)
	_ = l.ExecBatch(ctx, 1)
}

// lookalikes share a name with a statement runner and not its parameters, or its parameters and not
// its name.
type lookalike struct{}

func (lookalike) Query(ctx context.Context, n int) error { return nil }

func (lookalike) Exec(sql string) error { return nil }

func lookalikes(ctx context.Context, u *url.URL, l lookalike, conn *pgx.Conn) {
	_ = u.Query()
	_ = l.Query(ctx, 1)
	_ = l.Exec("SELECT 1")
	_ = conn.PgConn()
	_ = statements.Secret(ctx, nil)
}
