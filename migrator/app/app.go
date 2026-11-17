// Package app is the migrator's entry package and composition root, which migrator/main.go calls
// (ADR-0040, ADR-0139). It hands the host the migrator's configuration and its one piece of work, and
// the host runs it as a process that runs once (ADR-0138). The host starts it as it starts every
// process, refusing the database driver's password variables before the configuration is read,
// reading the configuration through the configuration library, answering --help and --version,
// logging every effective value, setting the level from log_level, validating and writing the start
// banner, and opens its pool. The work connects as the migrator's user the database section names, with
// the password from the mounted file it names, and applies the migration chain its binary carries
// under goose's advisory lock, its grants made to the application user application_user names
// (ADR-0048, ADR-0078, ADR-0079, ADR-0122, ADR-0143).
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
)

// Run is the migrator's process. It applies every migration of the chain the database does not yet
// hold, and returns once the chain is applied, or with the error that stopped it.
//
// logger is the logger main.go built over level, at info until the host sets level from log_level
// once the effective configuration is written, so goose's records follow it (ADR-0122). stdout answers
// --help and --version, and stderr is where the start banner goes, standard error in main.go, so
// standard output carries only the logger's JSON records otherwise.
func Run(ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	return host.RunOnce(ctx, args, environ, stdout, stderr, logger, level, host.Once[config.Migrator]{
		Name:      "mediated-mailbox-migrator",
		Defaults:  config.DefaultMigrator(runtime.NumCPU()),
		Process:   func(c config.Migrator) config.Process { return c.Process },
		Validate:  config.Migrator.Check,
		Component: "migrator",
		Work:      apply,
	})
}

// apply connects as the user the database section names and applies the chain this build carries,
// its grants made to the application user.
func apply(ctx context.Context, provided host.Provided[config.Migrator]) (err error) {
	pool, err := provided.OpenPool(ctx, provided.Loaded.Config.Database, provided.Registerer)
	if err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(pool)
	defer func() { err = errors.Join(err, db.Close()) }()
	return migrate.Apply(ctx, db, migrations.Chain(), provided.Loaded.Config.ApplicationUser, provided.Logger)
}
