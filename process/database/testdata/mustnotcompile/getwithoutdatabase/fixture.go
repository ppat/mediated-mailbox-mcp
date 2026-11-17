// Package getwithoutdatabase asks for a pool from a configuration type with no database part, which
// must not compile.
package getwithoutdatabase

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
)

// noDatabase holds the process part and no database part.
type noDatabase struct {
	config.Process
}

func Build(ctx context.Context, p process.Capabilities, c noDatabase) (*pgxpool.Pool, error) {
	return database.Get(ctx, p, c)
}
