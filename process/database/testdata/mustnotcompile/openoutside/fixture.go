// Package openoutside opens a pool from a deployable's configuration through the opening every pool
// goes through, which runs no schema check, so it must not compile, since that opening is the
// package's own.
package openoutside

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
)

func Build(ctx context.Context, p process.Capabilities, c config.API) (*pgxpool.Pool, error) {
	return database.Open(ctx, c.DatabasePart(), prometheus.NewRegistry())
}
