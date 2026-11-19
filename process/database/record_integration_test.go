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
	"github.com/jackc/pgx/v5/pgxpool"

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

// probe asks the running process's probe at path, whose address the serving record names, and
// returns its status and its body.
func probe(t *testing.T, logs func() string, path string) (int, string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for line := range strings.Lines(logs()) {
			if !strings.Contains(line, `"msg":"serving"`) {
				continue
			}
			_, rest, _ := strings.Cut(line, `"probe_listen":"`)
			addr, _, _ := strings.Cut(rest, `"`)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+path, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("asking %s: %v", path, err)
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

// await asks the readiness probe until it answers status with body in it, for up to within, and fails
// the test with the last answer otherwise.
func await(t *testing.T, logs func() string, within time.Duration, status int, body string) {
	t.Helper()
	var gotStatus int
	var gotBody string
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		gotStatus, gotBody = probe(t, logs, "/readyz")
		if gotStatus == status && strings.Contains(gotBody, body) {
			return
		}
	}
	t.Fatalf("the readiness probe answers %d %q, want %d with %q", gotStatus, gotBody, status, body)
}

// serveOver runs a process of one endpoint over the database url names until the test ends, and
// returns its log and the pool its endpoint got.
func serveOver(t *testing.T, url string) (*lockedBuffer, *pgxpool.Pool) {
	t.Helper()
	var logs lockedBuffer
	var pool *pgxpool.Pool
	built := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		args := append(flags(t, url), "--listen=127.0.0.1:0", "--log_level=debug")
		done <- process.RunContext(ctx, args, nil, &logs, io.Discard, process.Spec[served]{
			Name: "test", Endpoints: []process.Endpoint[served]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, c served) (http.Handler, error) {
				var err error
				pool, err = database.Get(ctx, p, c)
				close(built)
				return http.NotFoundHandler(), err
			}}},
		})
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	select {
	case <-built:
	case err := <-done:
		done <- err
		t.Fatalf("the process stopped before it was built: %v\n%s", err, logs.String())
	}
	return &logs, pool
}

// The process's one pool reports under database from the work run on it, and nothing else. It is
// ready once kept. Shut out from the database, it stays ready while nothing runs on it, since nothing
// shows a fault. A statement that then fails reports it not ready, and once the database lets
// connections in again the pool's own attempt to connect again reports it ready with no statement
// run, as it must for a process that takes no traffic while not ready. Throughout, the health probe
// answers 200, since a database that does not answer is no fault a restart clears within the bound
// (ADR-0142).
func TestThePoolsReportFollowsTheWorkRunOnIt(t *testing.T) {
	url := postgres.URL(t)
	logs, pool := serveOver(t, url)
	await(t, logs.String, 5*time.Second, http.StatusOK, "ready")
	open := shutOut(t, url)
	time.Sleep(time.Second)
	if status, body := probe(t, logs.String, "/readyz"); status != http.StatusOK {
		t.Errorf("shut out with nothing run, the readiness probe answers %d %q, want 200 on no evidence of a fault", status, body)
	}
	if _, err := pool.Exec(t.Context(), "SELECT 1"); err == nil {
		t.Fatal("a statement ran while the database refused every connection")
	}
	await(t, logs.String, 5*time.Second, http.StatusServiceUnavailable, "not ready: database: the database could not serve the latest statement or connection attempt")
	if status, body := probe(t, logs.String, "/healthz"); status != http.StatusOK {
		t.Errorf("while the database refuses connections the health probe answers %d %q, want 200", status, body)
	}
	open()
	await(t, logs.String, 25*time.Second, http.StatusOK, "ready")
}

// relay is a TCP relay between the process and the test database, which the test can pause, and
// which counts the bytes it carries. While paused it carries no byte either way, on the connections
// it holds and on those it accepts, so a statement or a connection attempt through it waits as one to
// a database that stopped answering waits.
type relay struct {
	t      *testing.T
	ln     net.Listener
	target string
	mu     sync.Mutex
	paused bool
	bytes  int
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
			r.mu.Lock()
			r.bytes += n
			r.mu.Unlock()
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

// carried returns how many bytes the relay has carried either way.
func (r *relay) carried() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytes
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

// refuseNew refuses every new connection to the database url names, leaving the connections open to
// it in place, and returns the function that lets new connections in again.
func refuseNew(t *testing.T, url string) func() {
	t.Helper()
	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{strings.TrimPrefix(u.Path, "/")}.Sanitize()
	admin, err := pgx.Connect(t.Context(), os.Getenv(postgres.EnvAdminURL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := admin.Exec(t.Context(), "ALTER DATABASE "+name+" ALLOW_CONNECTIONS false"); err != nil {
		t.Fatal(err)
	}
	opened := false
	open := func() {
		if !opened {
			opened = true
			if _, err := admin.Exec(context.Background(), "ALTER DATABASE "+name+" ALLOW_CONNECTIONS true"); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(open)
	return open
}

// A connection attempt that fails while the pool holds another connection shows no fault, since the
// pool can still serve on what it holds, so the process stays ready with no statement run after it.
// The pool's connections are closed in order, one is acquired and held, new connections are refused
// while the held one stays open, and a statement that needs a second connection fails to make it.
// Were the attempt a fault, nothing would clear it: the pool holds its one kept connection, so its
// maintenance attempts none, and nothing runs on it (ADR-0142).
func TestAFailedAttemptBesideAHeldConnectionShowsNoFault(t *testing.T) {
	url := postgres.URL(t)
	logs, pool := serveOver(t, url)
	await(t, logs.String, 5*time.Second, http.StatusOK, "ready")
	pool.Reset()
	time.Sleep(500 * time.Millisecond)
	held, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	open := refuseNew(t, url)
	if _, err := pool.Exec(t.Context(), "SELECT 1"); err == nil {
		t.Fatal("a statement made a second connection while the database refused new ones")
	}
	held.Release()
	open()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if status, body := probe(t, logs.String, "/readyz"); status != http.StatusOK {
			t.Fatalf("after a failed attempt beside a held connection the readiness probe answers %d %q, want 200", status, body)
		}
	}
}

// The probes run no statement, no ping and no call to the database. A process whose pool keeps a
// connection open through a relay is asked its probes many times over longer than any interval a
// report used to ping on, and the relay carries not one byte while it is asked (ADR-0142).
func TestAProbeRunsNoCallToTheDatabase(t *testing.T) {
	r, url := newRelay(t, postgres.URL(t))
	logs, _ := serveOver(t, url)
	await(t, logs.String, 5*time.Second, http.StatusOK, "ready")
	time.Sleep(time.Second)
	before := r.carried()
	for deadline := time.Now().Add(7 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		probe(t, logs.String, "/readyz")
		probe(t, logs.String, "/healthz")
	}
	if after := r.carried(); after != before {
		t.Errorf("the relay carried %d bytes while the probes were asked, want none", after-before)
	}
}

// A connection attempt the network never answers ends at its bound as a failed attempt, which
// reports the pool not ready, rather than waiting on with nothing recorded. The pool's connections
// are closed in order, so their end shows nothing, the relay stops carrying bytes, and a statement
// then waits on a new connection through the stalled relay with a deadline of its own far beyond
// the attempt's bound (ADR-0142).
func TestAConnectionAttemptThatIsNeverAnsweredIsAFault(t *testing.T) {
	r, url := newRelay(t, postgres.URL(t))
	logs, pool := serveOver(t, url)
	await(t, logs.String, 5*time.Second, http.StatusOK, "ready")
	pool.Reset()
	time.Sleep(500 * time.Millisecond)
	r.setPaused(true)
	defer r.setPaused(false)
	ctx, cancel := context.WithTimeout(t.Context(), 3*database.ConnectTimeout)
	defer cancel()
	if _, err := pool.Exec(ctx, "SELECT 1"); err == nil || ctx.Err() != nil {
		t.Fatalf("a statement through a relay that carries nothing ended with %v and its deadline %v, want a failed connection attempt before the deadline", err, ctx.Err())
	}
	await(t, logs.String, 5*time.Second, http.StatusServiceUnavailable, "not ready: database: ")
}

// A job kind's pool reports under the job kind's own name, so each job kind's pool is told apart on
// the readiness probe from the others and from the process's one pool, as the worker's readiness
// needs. A statement that fails on the second job kind's pool names it, though the first's reports
// first, and once the database lets connections in again the process is ready (ADR-0142).
func TestAJobKindsPoolIsReportedUnderItsOwnName(t *testing.T) {
	url := postgres.URL(t)
	var logs lockedBuffer
	pools := map[string]*pgxpool.Pool{}
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	kind := func(name string) process.Job[program] {
		return process.Job[program]{Name: name, Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			pool, err := database.Get(ctx, p, c)
			mu.Lock()
			pools[name] = pool
			mu.Unlock()
			return idle(), err
		}}
	}
	go func() {
		done <- process.RunContext(ctx, append(flags(t, url), "--log_level=debug"), nil, &logs, io.Discard, process.Spec[program]{
			Name: "test", Jobs: []process.Job[program]{kind("one"), kind("two")},
		})
	}()
	await(t, logs.String, 5*time.Second, http.StatusOK, "ready")
	open := shutOut(t, url)
	mu.Lock()
	two := pools["two"]
	mu.Unlock()
	if _, err := two.Exec(t.Context(), "SELECT 1"); err == nil {
		t.Fatal("a statement ran while the database refused every connection")
	}
	await(t, logs.String, 5*time.Second, http.StatusServiceUnavailable, "not ready: two: ")
	open()
	await(t, logs.String, 25*time.Second, http.StatusOK, "ready")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
