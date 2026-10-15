// Package migrate applies a migration chain to the database with goose linked as a library
// (ADR-0139). The migrator applies the chain its binary embeds, db/migrations, and the integration
// runs apply the same chain through the same code. It is the one package goose is admitted to.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Apply applies every migration of chain the database does not yet hold, in order, each in its own
// transaction, connected through db as the role that owns the schema. It first takes goose's
// PostgreSQL advisory lock on a connection of its own, so a second run at the same time waits, then
// finds the migrations already applied and applies nothing. goose logs each migration it applies
// through logger.
func Apply(ctx context.Context, db *sql.DB, chain fs.FS, logger *slog.Logger) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("building the migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, chain,
		goose.WithSessionLocker(locker),
		goose.WithSlog(logger),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("reading the migration chain: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("applying the migration chain: %w", err)
	}
	return nil
}
