//go:build integration

package sessions_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// role is the user the process connects as in these tests, the application user every program
// connects as (ADR-0143), so its grants apply as they do in production.
const role = postgres.ApplicationUser

// program is a configuration type composed as a program's that holds the Session, the process's parts
// and the accounts part inline, and the database part.
type program struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
	config.Accounts   `yaml:",inline"`
	Database          config.Database `yaml:"database"`
}

func (p program) Check() error { return p.Database.Check() }

func (program) Defaults(int) program {
	return program{
		Process: config.DefaultProcess(), Operations: config.DefaultOperations(), Accounts: config.DefaultAccounts(),
		Database: config.DefaultDatabase(4),
	}
}

func (p program) DatabasePart() config.Database { return p.Database }

// superuser returns a connection that holds every privilege, to write what the UI's setups and the
// operator would write, and to break what the tests break.
func superuser(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

// must runs a statement and fails the test on an error.
func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// revoke revokes a grant of role, and grants it again when the test ends.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+role)
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+role); err != nil {
			t.Error(err)
		}
	})
}

// reset empties the tables the Session reads. Every test of the package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{"account_state", "policy_rules", "accounts"} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// listed writes an account.
func listed(t *testing.T, conn *pgx.Conn, id string) {
	t.Helper()
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id)
}

// baseRule writes a restricted rule of the base policy.
func baseRule(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
}

// lockedBuffer is a destination the process and the test write and read from different goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// applicationFlags returns the flags naming the test database, connected to as the application user.
// pgrun gives the user the superuser's password, so the password file holds the one postgres.URL
// carries.
func applicationFlags(t *testing.T) []string {
	t.Helper()
	u, err := neturl.Parse(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + role, "--database.sslmode=disable", "--database.password_file=" + path,
		"--probe_listen=127.0.0.1:0",
	}
}

// running is a process a test runs, with the Session each of its two job kinds got and its logs.
type running struct {
	sessions [2]*sessions.Session
	logs     *lockedBuffer
}

// session returns the Session the process's first job kind got.
func (r *running) session() *sessions.Session { return r.sessions[0] }

// start runs a process of two job kinds, each of whose Build gets the Session, and returns once both
// builds have. The process stops when the test ends. The Session's first reload runs once the process
// serves.
func start(t *testing.T) *running {
	t.Helper()
	return startWith(t, "--account_reload_interval=1h")
}

// startWith runs the process start runs with args beyond the flags naming the test database, which
// set the reload interval.
func startWith(t *testing.T, args ...string) *running {
	t.Helper()
	r := &running{logs: &lockedBuffer{}}
	built := make(chan error, 2)
	job := func(i int, name string) process.Job[program] {
		return process.Job[program]{Name: name, Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			s, err := sessions.Get(ctx, p, c)
			r.sessions[i] = s
			built <- err
			return idle(), err
		}}
	}
	spec := process.Spec[program]{Name: "test", Jobs: []process.Job[program]{job(0, "one"), job(1, "two")}}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	flags := append(applicationFlags(t), args...)
	go func() { done <- process.RunContext(ctx, flags, nil, r.logs, io.Discard, spec) }()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("the process ended with %v", err)
		}
	})
	for range 2 {
		select {
		case err := <-built:
			if err != nil {
				t.Fatalf("building a job kind: %v\n%s", err, r.logs.String())
			}
		case err := <-done:
			done <- err
			t.Fatalf("the process ended before its builds: %v\n%s", err, r.logs.String())
		}
	}
	return r
}

// idle is a plan that lists nothing.
func idle() schedule.Plan {
	return schedule.Plan{
		Keys: func(context.Context) ([]string, error) { return nil, nil },
		Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
	}
}

// probe asks the running process's operations endpoint at path, whose address the serving record
// names, and returns its status and body.
func (r *running) probe(t *testing.T, path string) (int, string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		addr := probeAddress(r.logs.String())
		if addr == "" {
			continue
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		if err := errors.Join(err, resp.Body.Close()); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(body)
	}
	t.Fatalf("the process wrote no serving record:\n%s", r.logs.String())
	return 0, ""
}

// probeAddress returns the address the serving record names for the probes, "" before it is written.
func probeAddress(logs string) string {
	for line := range strings.Lines(logs) {
		if !strings.Contains(line, `"msg":"serving"`) {
			continue
		}
		_, rest, _ := strings.Cut(line, `"probe_listen":"`)
		addr, _, _ := strings.Cut(rest, `"`)
		return addr
	}
	return ""
}

// waitReady waits until the readiness probe answers ready, failing the test after within.
func (r *running) waitReady(t *testing.T, within time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if status, _ := r.probe(t, "/readyz"); status == http.StatusOK {
			return
		}
	}
	_, body := r.probe(t, "/readyz")
	t.Fatalf("the process was not ready within %v: %s", within, body)
}
