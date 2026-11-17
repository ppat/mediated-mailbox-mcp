//go:build integration

package database_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// served is a configuration type composed as a program's that serves endpoints and connects.
type served struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
	config.Serving    `yaml:",inline"`
	Database          config.Database `yaml:"database"`
}

func (s served) Check() error { return s.Database.Check() }

func (served) Defaults(int) served {
	return served{
		Process: config.DefaultProcess(), Operations: config.DefaultOperations(), Serving: config.DefaultServing(),
		Database: config.DefaultDatabase(2),
	}
}

func (s served) DatabasePart() config.Database { return s.Database }

// shutOut refuses every new connection to the database url names and ends every connection open to
// it, as a database that stops answering does, and returns the function that lets connections in
// again. It acts as the superuser of the maintenance database, which no setting of the database itself
// refuses.
func shutOut(t *testing.T, url string) func() {
	t.Helper()
	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{strings.TrimPrefix(u.Path, "/")}
	admin, err := pgx.Connect(t.Context(), os.Getenv(postgres.EnvAdminURL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec("ALTER DATABASE " + name.Sanitize() + " ALLOW_CONNECTIONS false")
	exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name[0])
	opened := false
	open := func() {
		if !opened {
			opened = true
			exec("ALTER DATABASE " + name.Sanitize() + " ALLOW_CONNECTIONS true")
		}
	}
	t.Cleanup(open)
	return open
}

// readiness asks the running process's readiness probe, whose address the serving record names, and
// returns its status and its body.
func readiness(t *testing.T, logs func() string) (int, string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for line := range strings.Lines(logs()) {
			if !strings.Contains(line, `"msg":"serving"`) {
				continue
			}
			_, rest, _ := strings.Cut(line, `"probe_listen":"`)
			addr, _, _ := strings.Cut(rest, `"`)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/readyz", http.NoBody)
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
	}
	t.Fatalf("the process wrote no serving record:\n%s", logs())
	return 0, ""
}

// await asks the readiness probe until it answers status, for up to the report's interval and its
// timeout together with a margin, and fails the test with the last answer otherwise.
func await(t *testing.T, logs func() string, status int, body string) {
	t.Helper()
	var gotStatus int
	var gotBody string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		gotStatus, gotBody = readiness(t, logs)
		if gotStatus == status && strings.Contains(gotBody, body) {
			return
		}
	}
	t.Fatalf("the readiness probe answers %d %q, want %d with %q", gotStatus, gotBody, status, body)
}

// The readiness report an endpoint opts the process's pool into is not ready until the database
// first answers a ping, ready while it answers, and not ready again once it stops, named database on
// the readiness probe. Asked for twice, it is one report, so each failed ping is logged once. The database is shut
// out after the pool's schema check, before the report starts, so its first ping fails (ADR-0142).
func TestThePingReportFollowsTheDatabase(t *testing.T) {
	url := postgres.URL(t)
	var open func()
	var logs lockedBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		args := append(flags(t, url), "--listen=127.0.0.1:0", "--log_level=debug")
		done <- process.RunContext(ctx, args, nil, &logs, io.Discard, process.Spec[served]{
			Name: "test", Endpoints: []process.Endpoint[served]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, c served) (http.Handler, error) {
				if _, err := database.Get(ctx, p, c); err != nil {
					return nil, err
				}
				open = shutOut(t, url)
				for range 2 {
					if err := database.ReportReadiness(ctx, p, c); err != nil {
						return nil, err
					}
				}
				return http.NotFoundHandler(), nil
			}}},
		})
	}()
	await(t, logs.String, http.StatusServiceUnavailable, "not ready: database: ")
	// A second report would ping beside the first and log its failure within the same interval.
	time.Sleep(time.Second)
	if n := strings.Count(logs.String(), `"msg":"the database did not answer a ping"`); n != 1 {
		t.Errorf("the first interval logged %d failed pings, want 1 from one report:\n%s", n, logs.String())
	}
	open()
	await(t, logs.String, http.StatusOK, "")
	shutOut(t, url)
	await(t, logs.String, http.StatusServiceUnavailable, "not ready: database: the database does not answer a ping")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// relay is a TCP relay between the process and the test database, which the test can pause. While
// paused it carries no byte either way, on the connections it holds and on those it accepts, so a
// ping through it waits as a ping to a database that stopped answering waits.
type relay struct {
	t      *testing.T
	ln     net.Listener
	target string
	mu     sync.Mutex
	paused bool
	open   []net.Conn
	// failed is the first error closing a connection gave, other than its being closed already.
	failed error
}

// newRelay starts a relay to the database url names and returns it with url pointed through it.
func newRelay(t *testing.T, url string) (*relay, string) {
	t.Helper()
	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &relay{t: t, ln: ln, target: u.Host}
	t.Cleanup(r.close)
	go r.accept()
	u.Host = ln.Addr().String()
	return r, u.String()
}

func (r *relay) accept() {
	for {
		client, err := r.ln.Accept()
		if err != nil {
			return
		}
		var d net.Dialer
		server, err := d.DialContext(context.Background(), "tcp", r.target)
		if err != nil {
			r.closeConn(client)
			continue
		}
		r.mu.Lock()
		r.open = append(r.open, client, server)
		r.mu.Unlock()
		go r.carry(server, client)
		go r.carry(client, server)
	}
}

// carry copies from src to dst, holding each chunk while the relay is paused.
func (r *relay) carry(dst, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		for r.isPaused() {
			time.Sleep(10 * time.Millisecond)
		}
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			r.closeConn(dst)
			return
		}
	}
}

func (r *relay) isPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

func (r *relay) setPaused(paused bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paused = paused
}

// closeConn closes c and keeps the first error that is not its being closed already.
func (r *relay) closeConn(c io.Closer) {
	if err := c.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.failed == nil {
			r.failed = err
		}
	}
}

func (r *relay) close() {
	r.setPaused(false)
	r.closeConn(r.ln)
	r.mu.Lock()
	open := r.open
	r.mu.Unlock()
	for _, c := range open {
		r.closeConn(c)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed != nil {
		r.t.Errorf("the relay could not close a connection: %v", r.failed)
	}
}

// The ping report reads not ready from the moment it is asked for until a ping answers, so a process
// whose first ping has not answered yet is not ready though it serves. The database is reached through
// a relay the test pauses after the pool's schema check, so the first ping cannot answer before the
// process serves, and the readiness probe is asked while it waits (ADR-0142).
func TestThePingReportIsNotReadyUntilAPingAnswers(t *testing.T) {
	r, url := newRelay(t, postgres.URL(t))
	var logs lockedBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		args := append(flags(t, url), "--listen=127.0.0.1:0")
		done <- process.RunContext(ctx, args, nil, &logs, io.Discard, process.Spec[served]{
			Name: "test", Endpoints: []process.Endpoint[served]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, c served) (http.Handler, error) {
				if _, err := database.Get(ctx, p, c); err != nil {
					return nil, err
				}
				r.setPaused(true)
				if err := database.ReportReadiness(ctx, p, c); err != nil {
					return nil, err
				}
				return http.NotFoundHandler(), nil
			}}},
		})
	}()
	status, body := readiness(t, logs.String)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "not ready: database: no ping has answered yet") {
		t.Errorf("while the first ping waits, the readiness probe answers %d %q, want 503 naming the database", status, body)
	}
	r.setPaused(false)
	await(t, logs.String, http.StatusOK, "")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// A job kind's part is refused the ping report, since a job kind's pool is its own and the report
// names the process's one pool, so the refusal fails the job kind's build and the start (ADR-0142).
func TestAJobKindIsRefusedThePingReport(t *testing.T) {
	// A report that let the job kind through would leave the process serving, so a deadline ends the
	// run.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := process.RunContext(ctx, flags(t, postgres.URL(t)), nil, io.Discard, io.Discard, process.Spec[program]{
		Name: "test", Jobs: []process.Job[program]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			return idle(), database.ReportReadiness(ctx, p, c)
		}}},
	})
	if err == nil || !strings.Contains(err.Error(), "a job kind's pool has no ping report yet") {
		t.Errorf("a job kind asking for the ping report ran to %v, want its build refused", err)
	}
}
