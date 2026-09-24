package postgres

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v5"
)

// MigrationRole is the role db/bootstrap/roles.sql creates to own the schema and run the chain.
const MigrationRole = "mediated_mailbox_migrate"

// ApplyChain applies a migration chain from an empty database, as pgrun does for every test run
// (ADR-0048). It creates the database name owned by the migration role, runs the per-database
// bootstrap file in it as the superuser, and runs goose up over the chain in dir as the migration
// role. admin is a superuser URL and migrate a URL connecting as the migration role, and ApplyChain
// points both at name. goose writes its progress to stderr, and a migration that fails fails the
// whole application.
func ApplyChain(ctx context.Context, admin, migrate, name, bootstrap, dir string) (err error) {
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close(context.Background())) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" OWNER "+pgx.Identifier{MigrationRole}.Sanitize()); err != nil {
		return err
	}

	databaseAdmin, err := inDatabase(admin, name)
	if err != nil {
		return err
	}
	dconn, err := pgx.Connect(ctx, databaseAdmin)
	if err != nil {
		return err
	}
	err = ExecFile(ctx, dconn, bootstrap)
	if closeErr := dconn.Close(context.Background()); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	databaseMigrate, err := inDatabase(migrate, name)
	if err != nil {
		return err
	}
	goose := exec.CommandContext(ctx, "goose", "-dir", dir, "postgres", databaseMigrate, "up")
	goose.Env = append(os.Environ(), "GOOSE_DRIVER=", "GOOSE_DBSTRING=", "GOOSE_MIGRATION_DIR=")
	goose.Stdout, goose.Stderr = os.Stderr, os.Stderr
	if err := goose.Run(); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// ExecFile runs a SQL file through the simple protocol, which accepts several statements at once.
func ExecFile(ctx context.Context, conn *pgx.Conn, path string) error {
	sql, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// inDatabase returns url with its database replaced by name.
func inDatabase(url, name string) (string, error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}
