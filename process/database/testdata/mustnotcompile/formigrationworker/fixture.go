// Package formigrationworker opens the migrator's pool from the Worker's configuration, which must not
// compile, since only the migrator's configuration type can ask for a pool with no schema check.
package formigrationworker

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
)

func Build(ctx context.Context, p process.Capabilities, c config.Worker) (*pgxpool.Pool, error) {
	return database.ForMigration(ctx, p, c)
}
