// Package postgres gives each integration test package its own database in the one PostgreSQL
// container a test run starts.
//
// testsupport/cmd/pgrun starts the container, applies the bootstrap and the migration chain once into
// a template database, and runs the test command with the variables below set. An integration test
// package calls Main from its TestMain, which creates the package's database from the template, so
// package test binaries running in parallel never share a database. Without the variables, Main fails
// the package rather than skipping it, because a skipped integration test passes in the job that was
// meant to run it.
package postgres

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// The contract between pgrun and Main.
const (
	// EnvAdminURL is a superuser connection URL for the container's maintenance database.
	EnvAdminURL = "PGRUN_ADMIN_URL"
	// EnvTemplate names the template database holding the applied chain.
	EnvTemplate = "PGRUN_TEMPLATE"
	// RegistryTable, in the maintenance database, gets one row per package database Main creates.
	// pgrun fails a run that registered none, which is what a run missing the integration tag does.
	RegistryTable = "pgrun_databases"
)

// url is this package's database, set by Main.
var url string

// Main creates this test package's database from the template, runs the tests, drops the database,
// and exits. Call it from TestMain.
func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	admin, template := os.Getenv(EnvAdminURL), os.Getenv(EnvTemplate)
	if admin == "" || template == "" {
		fmt.Fprintf(os.Stderr, "integration tests need the database pgrun starts, and %s or %s is not set.\nRun them as: go tool pgrun -- go test -tags integration ./...\n", EnvAdminURL, EnvTemplate)
		return 1
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connecting to the database pgrun started: %v\n", err)
		return 1
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "closing the connection: %v\n", err)
		}
	}()

	name := databaseName(dir, os.Getpid())
	database := pgx.Identifier{name}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+database+" TEMPLATE "+pgx.Identifier{template}.Sanitize()); err != nil {
		fmt.Fprintf(os.Stderr, "creating %s from the template: %v\n", name, err)
		return 1
	}
	defer func() {
		if _, err := conn.Exec(context.Background(), "DROP DATABASE "+database+" WITH (FORCE)"); err != nil {
			fmt.Fprintf(os.Stderr, "dropping %s: %v\n", name, err)
		}
	}()
	if _, err := conn.Exec(ctx, "INSERT INTO "+RegistryTable+" (package_dir, database) VALUES ($1, $2)", dir, name); err != nil {
		fmt.Fprintf(os.Stderr, "registering %s: %v\n", name, err)
		return 1
	}
	u, err := neturl.Parse(admin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s is not a URL: %v\n", EnvAdminURL, err)
		return 1
	}
	u.Path = "/" + name
	url = u.String()
	return m.Run()
}

var unsafe = regexp.MustCompile(`[^a-z0-9]+`)

// databaseName is unique per test binary, and readable in the registry and in server logs.
func databaseName(dir string, pid int) string {
	base := unsafe.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "_")
	return fmt.Sprintf("test_%.40s_%d", base, pid)
}

// URL returns a superuser connection string for this package's database. It fails the test when Main
// did not run, so a package that forgot its TestMain fails rather than reaching another database.
func URL(tb testing.TB) string {
	tb.Helper()
	if url == "" {
		tb.Fatal("postgres.Main did not run. Call it from this package's TestMain")
	}
	return url
}
