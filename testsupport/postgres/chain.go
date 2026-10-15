package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/ppat/mediated-mailbox-mcp/db/migrate"
)

// MigrationRole is the role db/bootstrap/roles.sql creates to own the schema and run the chain.
const MigrationRole = "mediated_mailbox_migrator"

// ApplyChain applies a migration chain from an empty database, as pgrun does for every test run
// (ADR-0048). It creates the database name owned by the migration role and applies chain as the
// migration role through db/migrate, the code the migrator applies the chain with (ADR-0139). Nothing
// runs in the database before the chain, since the schema needs only trusted extensions, which the
// chain's first migration creates as the migration role. admin is a superuser URL and migrator a URL
// connecting as the migration role, and ApplyChain points both at name. goose logs its progress to
// stderr, and a migration that fails fails the whole application.
func ApplyChain(ctx context.Context, admin, migrator, name string, chain fs.FS) (err error) {
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close(context.Background())) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" OWNER "+pgx.Identifier{MigrationRole}.Sanitize()); err != nil {
		return err
	}

	url, err := inDatabase(migrator, name)
	if err != nil {
		return err
	}
	return up(ctx, chain, url)
}

// Upgrade is a database the migration chain was applied to from empty, up to and not including one
// migration, so a test stores rows under the chain before that migration and then applies it, as a
// migration that touches system-of-record data must be tested (ADR-0048).
type Upgrade struct {
	// URL connects to the database as the migration role, which owns every table.
	URL   string
	chain fs.FS
}

// ChainBefore applies the chain in dir from empty into a database of the test's own, every migration
// numbered below version, as ApplyChain does, and drops the database when the test ends. It needs the
// variables pgrun sets.
func ChainBefore(tb testing.TB, dir string, version int64) *Upgrade {
	tb.Helper()
	admin := os.Getenv(EnvAdminURL)
	if admin == "" {
		tb.Fatalf("run under pgrun, which sets %s", EnvAdminURL)
	}
	migrations, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(migrations) == 0 {
		tb.Fatalf("no migrations found in %s: %v", dir, err)
	}
	before := tb.TempDir()
	found := false
	for _, path := range migrations {
		var v int64
		if _, err := fmt.Sscanf(filepath.Base(path), "%d_", &v); err != nil {
			tb.Fatalf("%s: %v", path, err)
		}
		found = found || v == version
		if v >= version {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		//nolint:gosec // The name is a checked-in migration's, written into the test's own directory.
		if err := os.WriteFile(filepath.Join(before, filepath.Base(path)), src, 0o600); err != nil {
			tb.Fatal(err)
		}
	}
	if !found {
		tb.Fatalf("no migration numbered %d in %s", version, dir)
	}

	migrator, err := neturl.Parse(admin)
	if err != nil {
		tb.Fatal(err)
	}
	// The superuser connection acting as the migration role, whose password pgrun does not export. A
	// space encoded as a plus sign reaches the server as a plus sign.
	migrator.RawQuery += "&options=" + strings.ReplaceAll(neturl.QueryEscape("-c role="+MigrationRole), "+", "%20")
	name := fmt.Sprintf("upgrade_%d_%d", version, os.Getpid())
	tb.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), admin)
		if err != nil {
			tb.Error(err)
			return
		}
		if _, err := conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			tb.Error(err)
		}
		if err := conn.Close(context.Background()); err != nil {
			tb.Error(err)
		}
	})
	if err := ApplyChain(tb.Context(), admin, migrator.String(), name, os.DirFS(before)); err != nil {
		tb.Fatal(err)
	}
	url, err := inDatabase(migrator.String(), name)
	if err != nil {
		tb.Fatal(err)
	}
	return &Upgrade{URL: url, chain: os.DirFS(dir)}
}

// Up applies the whole chain to the database, every migration not yet applied, and returns the error
// that stopped it, which names the migration and the database's error, so a test can require a
// migration to stop and on what.
func (u *Upgrade) Up(ctx context.Context) error {
	return up(ctx, u.chain, u.URL)
}

// up applies chain to the database url names, through db/migrate.
func up(ctx context.Context, chain fs.FS, url string) (err error) {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*config)
	defer func() { err = errors.Join(err, db.Close()) }()
	return migrate.Apply(ctx, db, chain, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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
