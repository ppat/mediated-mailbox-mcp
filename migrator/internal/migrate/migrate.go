// Package migrate is the migrator's one task, which applies the migration chain its binary carries
// (ADR-0048, ADR-0139). It connects as the migrator's user the database part names, with the password
// from the mounted file it names, and applies every migration the database does not yet hold through
// db/migrate under goose's advisory lock, its grants made to the application user application_user
// names (ADR-0078, ADR-0079, ADR-0143). It sits in the migrator's own code because a task's Build takes
// the process's capabilities, and the layers bar db/ from importing the process family (ADR-0142).
package migrate

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
)

// Build opens the migrator's pool, with no schema check, since the migrator moves the schema, and
// returns the work that applies the chain this build carries. goose logs through the logger the
// process hands the task, which carries the program's name (ADR-0122).
func Build(ctx context.Context, p process.Capabilities, c config.Migrator) (func(context.Context) error, error) {
	pool, err := database.ForMigration(ctx, p, c)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) (err error) {
		db := stdlib.OpenDBFromPool(pool)
		defer func() { err = errors.Join(err, db.Close()) }()
		return migrate.Apply(ctx, db, migrations.Chain(), c.ApplicationUser, p.Logger())
	}, nil
}
