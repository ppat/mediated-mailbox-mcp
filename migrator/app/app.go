// Package app is the migrator's entry package and composition root, which migrator/main.go calls
// (ADR-0040, ADR-0139). It hands the host the migrator's configuration and its one piece of work, and
// the host runs it as a process that runs once (ADR-0138). The host starts it as it starts every
// process, refusing the database driver's password variables before the configuration is read,
// reading the configuration through the configuration library, answering --help and --version,
// logging every effective value, setting the level from log_level, validating and writing the start
// banner, and opens its pool. The work connects as the migration role the database section names, with
// the password from the mounted file it names, and applies the migration chain its binary carries
// under goose's advisory lock (ADR-0078, ADR-0079, ADR-0122).
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
)

// Configuration is the migrator's root configuration type, loaded by the configuration library
// (ADR-0078). It composes the process's section, inline at the top of the tree, and the database
// section. Its fields are pinned by the test beside this file, so a new value is a visible change.
type Configuration struct {
	host.Process `yaml:",inline"`
	Database     dbconnectcore.Config `yaml:"database"`
}

// defaults are the migrator's defaults. The user is the migration role, which owns the schema
// (ADR-0048), the TLS mode is the one that fails closed, and the pool size is the driver's own default.
// The password file has no default, since a default path assumes the environment.
func defaults() Configuration {
	return Configuration{
		Process:  host.DefaultProcess(),
		Database: dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_migrator", SSLMode: "verify-full", PoolSize: dbconnect.DefaultPoolSize()},
	}
}

// validate refuses a database section the migrator cannot connect with.
func validate(c Configuration) error {
	return dbconnectcore.Validate(c.Database)
}

// Run is the migrator's process. It applies every migration of the chain the database does not yet
// hold, and returns once the chain is applied, or with the error that stopped it.
//
// logger is the logger main.go built over level, at info until the host sets level from log_level
// once the effective configuration is written, so goose's records follow it (ADR-0122). stdout answers
// --help and --version, and stderr is where the start banner goes, standard error in main.go, so
// standard output carries only the logger's JSON records otherwise.
func Run(ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	return host.RunOnce(ctx, args, environ, stdout, stderr, logger, level, host.Once[Configuration]{
		Name:      "mediated-mailbox-migrator",
		Defaults:  defaults(),
		Process:   func(c Configuration) host.Process { return c.Process },
		Validate:  validate,
		Component: "migrator",
		Work:      apply,
	})
}

// apply connects as the role the database section names and applies the chain this build carries.
func apply(ctx context.Context, provided host.Provided[Configuration]) (err error) {
	pool, err := provided.OpenPool(ctx, provided.Loaded.Config.Database, provided.Registerer)
	if err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(pool)
	defer func() { err = errors.Join(err, db.Close()) }()
	return migrate.Apply(ctx, db, migrations.Chain(), provided.Logger)
}
