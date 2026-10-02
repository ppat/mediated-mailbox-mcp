//go:build banproof

package api

import (
	"context"
	"io"

	"github.com/jackc/pgx/v5/pgconn"
)

// This file reads a client's sealed secret through the database handle, through the driver's
// lower-level connection, and through an interface declared over that connection, on purpose. The UI's role may read the column for ui/internal/clientsecret
// alone, and the import lists confine that read only while every statement the UI runs is one the
// data-access library generated (ADR-0071, ADR-0081).
func secretByHand(ctx context.Context, s *Server) {
	row := s.opts.Database.QueryRow(ctx, "SELECT client_secret FROM oauth_clients") // want vetcheck "The method QueryRow runs a statement the data-access library did not generate"
	_ = row
}

func secretByProtocol(ctx context.Context, c *pgconn.PgConn) {
	result := c.ExecParams(ctx, "SELECT client_secret FROM oauth_clients", nil, nil, nil, nil) // want vetcheck "The method ExecParams runs a statement the data-access library did not generate"
	_ = result
}

// copier is an interface declared over the lower-level connection's copy out, which a *pgconn.PgConn
// satisfies.
type copier interface {
	CopyTo(ctx context.Context, w io.Writer, sql string) (pgconn.CommandTag, error)
}

func secretByCopy(ctx context.Context, c copier, w io.Writer) {
	_, err := c.CopyTo(ctx, w, "COPY oauth_clients (client_secret) TO STDOUT") // want vetcheck "The method CopyTo runs a statement the data-access library did not generate"
	_ = err
}
