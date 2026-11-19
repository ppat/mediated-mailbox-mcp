package bearer_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/bearer"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// lockedBuffer is the process's standard output, which the process writes and the test reads from
// different goroutines.
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

// tokenFile writes token to a file standing in for the mounted token and returns its path.
func tokenFile(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// running is a process serving the test's spec, with the addresses its serving record names.
type running struct {
	app, probes string
	logs        *lockedBuffer
	// calls counts the requests that reached either endpoint's handler behind the check.
	calls *atomic.Int64
}

// handler is what each endpoint serves behind the check. It counts each request that reaches it and
// sets its own caching headers, which the no-store header must override.
func handler(calls *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Query().Get("answer") {
		case "nothing":
		case "body":
			w.Header().Set("Cache-Control", "max-age=60")
			w.Header().Set("ETag", `"v2"`)
			if _, err := w.Write([]byte("{}")); err != nil {
				panic(err)
			}
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Header().Set("ETag", `"v1"`)
			w.WriteHeader(http.StatusOK)
		}
	})
}

// spec is a program shaped as the API server is, two endpoints at the API server's patterns, each
// serving handler behind bearer.Wrap. Its endpoints build nothing else, so it needs no database.
func spec(calls *atomic.Int64) process.Spec[config.API] {
	build := func(ctx context.Context, p process.Capabilities, c config.API) (http.Handler, error) {
		return bearer.Wrap(ctx, p, c, handler(calls))
	}
	return process.Spec[config.API]{
		Name:      "api",
		Endpoints: []process.Endpoint[config.API]{{Pattern: "/api/", Build: build}, {Pattern: "/mcp", Build: build}},
	}
}

// args are the flags a start of spec needs, with token the token file, each server on a loopback port
// the system picks. The database is never connected to, since nothing the spec builds asks for it.
func args(token string, more ...string) []string {
	return append([]string{
		"--token_file=" + token, "--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0",
		"--database.host=db.example", "--database.name=mailbox", "--database.user=mailbox_app",
		"--database.password_file=/absent/password",
		"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]",
	}, more...)
}

// start runs spec with args until the test ends, and returns once the process serves. A start that
// stops before it serves fails the test.
func start(t *testing.T, args []string) *running {
	t.Helper()
	r := &running{logs: &lockedBuffer{}, calls: &atomic.Int64{}}
	done := make(chan error, 1)
	ctx, stop := context.WithCancel(context.Background())
	go func() { done <- process.RunContext(ctx, args, nil, r.logs, io.Discard, spec(r.calls)) }()
	t.Cleanup(func() {
		stop()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("the process ended with %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("the process did not stop")
		}
	})
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("the process stopped before it served: %v\n%s", err, r.logs.String())
		default:
		}
		for line := range strings.Lines(r.logs.String()) {
			var rec struct {
				Msg         string `json:"msg"`
				Listen      string `json:"listen"`
				ProbeListen string `json:"probe_listen"`
			}
			if err := json.Unmarshal([]byte(line), &rec); err == nil && rec.Msg == "serving" {
				r.app, r.probes = rec.Listen, rec.ProbeListen
				return r
			}
		}
	}
	t.Fatalf("the process wrote no serving record:\n%s", r.logs.String())
	return nil
}

// request is one request to the process's application server.
type request struct {
	method, path, authorization, body string
}

// send sends q to the application server and returns the response's status and headers.
func (r *running) send(t *testing.T, q request) (int, http.Header) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, q.method, "http://"+r.app+q.path, strings.NewReader(q.body))
	if err != nil {
		t.Fatal(err)
	}
	if q.authorization != "" {
		req.Header.Set("Authorization", q.authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header
}

// ready asks the readiness probe and returns its status and body.
func (r *running) ready(t *testing.T) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+r.probes+"/readyz", http.NoBody)
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
	return resp.StatusCode, strings.TrimSpace(string(body))
}

// await asks the readiness probe until it answers status with a body holding want, for up to two of
// the report's intervals, and fails the test with the last answer otherwise.
func (r *running) await(t *testing.T, status int, want string) {
	t.Helper()
	var code int
	var body string
	for deadline := time.Now().Add(12 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if code, body = r.ready(t); code == status && strings.Contains(body, want) {
			return
		}
	}
	t.Fatalf("the readiness probe answered %d %q, want %d holding %q", code, body, status, want)
}

// The calls on each endpoint, the API server's two patterns.
var (
	apiCall = request{method: http.MethodGet, path: "/api/accounts/acct-a/echo"}
	mcpCall = request{method: http.MethodPost, path: "/mcp", body: `{}`}
)

func (q request) with(authorization string) request {
	q.authorization = authorization
	return q
}

// Every request to either endpoint passes the bearer check before it reaches the endpoint's handler,
// so a request without the token never reaches it (ADR-0030). A token file emptied or removed while
// the process runs admits nothing, whatever the request carries.
func TestEveryRequestPassesTheBearerCheckFirst(t *testing.T) {
	token := tokenFile(t, "s3cret\n")
	r := start(t, args(token))
	refused := map[string]request{
		"the API endpoint without a token":     apiCall,
		"the MCP endpoint without a token":     mcpCall,
		"the API endpoint with a wrong token":  apiCall.with("Bearer wrong"),
		"the MCP endpoint with a wrong token":  mcpCall.with("Bearer wrong"),
		"the token as a basic credential":      apiCall.with("Basic s3cret"),
		"the token without a scheme":           apiCall.with("s3cret"),
		"a token with the right one as prefix": apiCall.with("Bearer s3cret2"),
	}
	check := func(state string, cases map[string]request) {
		t.Helper()
		for name, q := range cases {
			if got, _ := r.send(t, q); got != http.StatusUnauthorized {
				t.Errorf("%s, %s: status %d, want 401", state, name, got)
			}
		}
		if n := r.calls.Load(); n != 0 {
			t.Errorf("%s: a handler ran %d times behind a refused bearer check", state, n)
		}
	}
	check("a token", refused)
	if err := os.WriteFile(token, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	check("an emptied token file", map[string]request{"an empty token": apiCall.with("Bearer "), "the old token": mcpCall.with("Bearer s3cret")})
	if err := os.Remove(token); err != nil {
		t.Fatal(err)
	}
	check("a removed token file", map[string]request{"an empty token": apiCall.with("Bearer "), "the old token": apiCall.with("Bearer s3cret")})
}

// With the token, both endpoints reach their handlers, the scheme read in any case. A path outside
// both patterns is not the check's, so the probes and the metrics endpoint are not served there.
func TestTheTokenReachesBothEndpoints(t *testing.T) {
	r := start(t, args(tokenFile(t, "s3cret\n")))
	got := map[string]int{}
	got["api"], _ = r.send(t, apiCall.with("Bearer s3cret"))
	got["mcp"], _ = r.send(t, mcpCall.with("bearer s3cret"))
	got["healthz"], _ = r.send(t, request{method: http.MethodGet, path: "/healthz", authorization: "Bearer s3cret"})
	got["metrics"], _ = r.send(t, request{method: http.MethodGet, path: "/metrics", authorization: "Bearer s3cret"})
	want := map[string]int{"api": 200, "mcp": 200, "healthz": 404, "metrics": 404}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("statuses (-want +got):\n%s", diff)
	}
	if n := r.calls.Load(); n != 2 {
		t.Errorf("the handlers ran %d times, want once per endpoint", n)
	}
}

// Every response either endpoint gives carries Cache-Control: no-store and no ETag, a refusal's and
// one whose handler set its own caching headers, wrote only its body or wrote nothing, so no response
// the gate decided outlives a change in its decision (ADR-0087).
func TestEveryResponseIsUncacheable(t *testing.T) {
	r := start(t, args(tokenFile(t, "s3cret")))
	for name, q := range map[string]request{
		"a handler writing its status":    apiCall.with("Bearer s3cret"),
		"a handler writing only its body": {method: http.MethodGet, path: "/api/x?answer=body", authorization: "Bearer s3cret"},
		"a handler writing nothing":       {method: http.MethodPost, path: "/mcp?answer=nothing", authorization: "Bearer s3cret"},
		"a refused bearer token":          apiCall,
		"a refused token on the MCP root": mcpCall,
	} {
		status, header := r.send(t, q)
		if got := header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s answered %d with Cache-Control %q", name, status, got)
		}
		if got := header.Values("ETag"); len(got) != 0 {
			t.Errorf("%s answered %d with ETag %q", name, status, got)
		}
	}
}

// A token file that cannot be read or holds no token admits no client, so it refuses the start
// before anything is served, and ready is never reported (ADR-0051, ADR-0127).
func TestATokenFileThatAdmitsNoClientRefusesTheStart(t *testing.T) {
	for name, c := range map[string]struct{ file, want string }{
		"absent":           {filepath.Join(t.TempDir(), "absent"), "reading the bearer token: "},
		"holding no token": {tokenFile(t, " \n"), "the bearer token file holds no token"},
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int64
			logs := &lockedBuffer{}
			// A start the token file does not refuse serves until the deadline, so the test ends either way.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			err := process.RunContext(ctx, args(c.file), nil, logs, io.Discard, spec(&calls))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("the run returned %v, want the start refused with %q", err, c.want)
			}
			if strings.Contains(logs.String(), `"msg":"serving"`) {
				t.Errorf("the process served:\n%s", logs.String())
			}
		})
	}
}

// The readiness report follows the token file while the process runs. It is ready while the file holds
// a token, not ready once the file is emptied, ready again once it holds one, and not ready once it is
// removed, each named on the probe as the bearer token's and naming no error's detail (ADR-0051,
// ADR-0142).
func TestTheReportFollowsTheTokenFile(t *testing.T) {
	token := tokenFile(t, "s3cret")
	r := start(t, args(token))
	r.await(t, http.StatusOK, "")
	if err := os.WriteFile(token, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.await(t, http.StatusServiceUnavailable, "not ready: bearer token: the bearer token file holds no token")
	if err := os.WriteFile(token, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.await(t, http.StatusOK, "")
	if err := os.Remove(token); err != nil {
		t.Fatal(err)
	}
	r.await(t, http.StatusServiceUnavailable, "not ready: bearer token: the bearer token file cannot be read")
	if _, body := r.ready(t); strings.Contains(body, "no such file") {
		t.Errorf("the probe names the read's error: %s", body)
	}
}

// The report reads not ready from the moment it starts until its first read of the file returns, so a
// process whose report has not yet read the file is not ready though it serves. The token file is a
// named pipe, so each read waits for the test to write to it: the start's own read is given the
// token, and the report's first read waits while the probe is asked (ADR-0142).
func TestTheReportIsNotReadyUntilItReadsTheFile(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "token")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	// write gives one reader of the pipe token and closes it, so that read returns.
	write := func(token string) error {
		f, err := os.OpenFile(pipe, os.O_WRONLY, 0) //nolint:gosec // the test's own pipe
		if err != nil {
			return err
		}
		_, err = f.WriteString(token)
		return errors.Join(err, f.Close())
	}
	// A write on its own goroutine waits in its open until a reader opens the pipe, and the process
	// may stop with none left to read it. Once the process has stopped, and before the pipe's directory
	// is removed, the test closes stopped so no further write begins, opens the pipe for reading
	// itself, an open that waits for no writer, so a writer still waiting is let through, and keeps it
	// open until every writer has ended, so none outlives the test or reports to it after it ends.
	// Cleanups run last registered first, so this one, registered before start's, runs after the
	// process has stopped.
	var writers sync.WaitGroup
	stopped := make(chan struct{})
	t.Cleanup(func() {
		close(stopped)
		f, err := os.OpenFile(pipe, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // the test's own pipe
		if err != nil {
			t.Error(err)
			return
		}
		writers.Wait()
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	writers.Go(func() {
		if err := write("s3cret"); err != nil {
			t.Error(err)
		}
	})
	r := start(t, args(pipe))
	status, body := r.ready(t)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "not ready: bearer token: the bearer token file has not been read yet") {
		t.Errorf("before its first read the readiness probe answered %d %q", status, body)
	}
	if err := write("s3cret"); err != nil {
		t.Error(err)
	}
	r.await(t, http.StatusOK, "")
	// The report may be waiting to read the pipe again when the process is told to stop, and may read
	// it once more after, so the test answers every read until the process has stopped.
	t.Cleanup(func() {
		writers.Go(func() {
			for {
				select {
				case <-stopped:
					return
				default:
				}
				// A read that ends between this write's open and its write leaves the pipe with no reader,
				// which fails no read, since the next write answers the next one.
				if err := write("s3cret"); err != nil && !errors.Is(err, syscall.EPIPE) {
					t.Error(err)
					return
				}
			}
		})
	})
}

// The two endpoints share one report, so one loop reads the file and each failed read is logged once
// per interval. The file is removed, and once the first failed read is logged no second is logged
// within the second after it, where a loop per endpoint would log one each (ADR-0142).
func TestBothEndpointsShareOneReport(t *testing.T) {
	token := tokenFile(t, "s3cret")
	r := start(t, args(token, "--log_level=debug"))
	r.await(t, http.StatusOK, "")
	if err := os.Remove(token); err != nil {
		t.Fatal(err)
	}
	failed := func() int { return strings.Count(r.logs.String(), "the bearer token file could not be read") }
	for deadline := time.Now().Add(12 * time.Second); failed() == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no failed read was logged")
		}
	}
	time.Sleep(time.Second)
	if n := failed(); n != 1 {
		t.Errorf("one interval's failed reads were logged %d times, want once", n)
	}
}
