// Command pgrun runs a command against one PostgreSQL container started for it.
//
// It starts the container with the ordinary docker command (ADR-0068), runs db/bootstrap as the
// superuser, applies the migration chain from empty with goose as the migration role into a template
// database, runs the command with the connection details exported, and removes the container. Each
// integration test package then creates its own database from the template (testsupport/postgres).
//
// A run where the command succeeds but no test package created a database fails, because that is
// what running the integration job without the integration tag looks like.
//
// Run it from the repository root. The command follows the flags.
//
//	go tool pgrun -- go test -tags integration ./...
//
// With a remote docker daemon, the published port is on the daemon's machine, so -host names an
// address that reaches it. The integration suite also needs the next port reachable, because one of
// its tests starts a second run of pgrun there (testsupport/postgres).
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

const template = "pgrun_template"

func main() {
	host := flag.String("host", "127.0.0.1", "address at which the container's published port is reached")
	port := flag.Int("port", 55432, "fixed host port the container publishes PostgreSQL on (ADR-0068)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: go tool pgrun [flags] -- command [arguments]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	code, err := run(*host, *port, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgrun:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func run(host string, port int, command []string) (int, error) {
	if _, err := os.Stat("go.mod"); err != nil {
		return 1, errors.New("run from the repository root, where go.mod is")
	}
	for _, tool := range []string{"docker", "goose", command[0]} {
		if _, err := exec.LookPath(tool); err != nil {
			return 1, fmt.Errorf("%s is not on PATH: %w", tool, err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	suffix := strings.ToLower(rand.Text()[:10])
	password := rand.Text()
	container := "pgrun-" + suffix
	started := time.Now()
	logf("starting %s from %s", container, image)
	bind := "127.0.0.1"
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		bind = "0.0.0.0"
	}
	run := exec.Command("docker", "run", "--detach", "--name", container,
		"--label", "io.github.ppat.mediated-mailbox-mcp.pgrun=true",
		"--env", "POSTGRES_PASSWORD="+password,
		"--publish", fmt.Sprintf("%s:%d:5432", bind, port),
		image)
	run.Stdout, run.Stderr = os.Stderr, os.Stderr
	if err := run.Run(); err != nil {
		return 1, fmt.Errorf("starting the container: %w", err)
	}
	// Removal runs whatever happens next, including an interrupt, which cancels ctx.
	defer func() {
		out, err := exec.Command("docker", "rm", "--force", "--volumes", container).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pgrun: removing %s failed, remove it by hand: %v\n%s", container, err, out)
		}
	}()

	admin := connURL("postgres", password, host, port, "postgres")
	if err := waitReady(ctx, admin, container); err != nil {
		return 1, err
	}
	logf("ready after %s", time.Since(started).Round(time.Millisecond))

	prepared := time.Now()
	if err := prepareTemplate(ctx, admin, connURL(postgres.MigrationRole, password, host, port, template), password); err != nil {
		return 1, err
	}
	logf("bootstrap and migration chain applied in %s", time.Since(prepared).Round(time.Millisecond))

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), postgres.EnvAdminURL+"="+admin, postgres.EnvTemplate+"="+template)
	ran := time.Now()
	cmdErr := cmd.Run()
	logf("command finished in %s", time.Since(ran).Round(time.Millisecond))
	var exit *exec.ExitError
	switch {
	case errors.As(cmdErr, &exit):
		return exit.ExitCode(), nil
	case cmdErr != nil:
		return 1, fmt.Errorf("running the command: %w", cmdErr)
	}

	registered, err := countRegistered(ctx, admin)
	if err != nil {
		return 1, err
	}
	if registered == 0 {
		return 1, errors.New("the command succeeded but no test package created a database, so no integration test ran. Pass -tags integration to go test")
	}
	logf("%d test packages ran against their own database, %s in all", registered, time.Since(started).Round(time.Millisecond))
	return 0, nil
}

func connURL(user, password, host string, port int, database string) string {
	u := neturl.URL{
		Scheme:   "postgres",
		User:     neturl.UserPassword(user, password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// waitReady waits until the server accepts connections through the published port. The image's
// entrypoint first runs a temporary server that listens on no TCP address, so a TCP connection only
// succeeds once the final server is up. A route that does not reach the port fails here, before any
// test runs.
func waitReady(ctx context.Context, admin, container string) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		attempt, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := pgx.Connect(attempt, admin)
		cancel()
		if err == nil {
			return conn.Close(ctx)
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			logs, logsErr := exec.Command("docker", "logs", "--tail", "20", container).CombinedOutput()
			if logsErr != nil {
				logs = fmt.Appendf(logs, "reading the container's logs: %v", logsErr)
			}
			return fmt.Errorf("the server was not reachable through the published port: %w\n%s", err, logs)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// prepareTemplate runs the bootstrap, applies the chain from empty as the migration role, and turns
// the result into a template no one connects to.
func prepareTemplate(ctx context.Context, admin, migrate, password string) error {
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return err
	}
	defer closeConn(conn)
	if err := postgres.ExecFile(ctx, conn, "db/bootstrap/roles.sql"); err != nil {
		return err
	}
	// The bootstrap sets no credentials, so the test run gives the migration role its own.
	if _, err := conn.Exec(ctx, "ALTER ROLE "+pgx.Identifier{postgres.MigrationRole}.Sanitize()+" PASSWORD '"+password+"'"); err != nil {
		return fmt.Errorf("the bootstrap must create the migration role %s: %w", postgres.MigrationRole, err)
	}
	if _, err := conn.Exec(ctx, "CREATE TABLE "+postgres.RegistryTable+" (package_dir text NOT NULL, database text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT clock_timestamp())"); err != nil {
		return err
	}
	if err := postgres.ApplyChain(ctx, admin, migrate, template, "db/bootstrap/extensions.sql", filepath.Join("db", "migrations")); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "ALTER DATABASE "+template+" WITH IS_TEMPLATE true ALLOW_CONNECTIONS false")
	return err
}

func countRegistered(ctx context.Context, admin string) (int, error) {
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return 0, err
	}
	defer closeConn(conn)
	var n int
	err = conn.QueryRow(ctx, "SELECT count(*) FROM "+postgres.RegistryTable).Scan(&n)
	return n, err
}

func closeConn(conn *pgx.Conn) {
	if err := conn.Close(context.Background()); err != nil {
		logf("closing a connection: %v", err)
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pgrun: "+format+"\n", args...)
}
