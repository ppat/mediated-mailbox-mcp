//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// policy is the content security policy ADR-0062 decides, written out here rather than read from the
// code under test, so a change to the constant turns this red (ADR-0046).
const policy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; frame-ancestors 'none'"

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

// running is the UI running from its spec as its main.go runs it, over the test database.
type running struct {
	t      *testing.T
	logs   *lockedBuffer
	app    string
	probes string
	stop   context.CancelFunc
	done   chan error
}

// start runs the UI from its spec until the test ends or stop is called, configured through the flags
// flags returns with its probes on an ephemeral loopback port, and returns once it serves.
func start(t *testing.T, url string) *running {
	t.Helper()
	args := flags(t, url, "127.0.0.1:0")
	ctx, cancel := context.WithCancel(t.Context())
	r := &running{t: t, logs: &lockedBuffer{}, stop: cancel, done: make(chan error, 1)}
	go func() { r.done <- run(ctx, args, nil, r.logs, io.Discard) }()
	t.Cleanup(func() {
		cancel()
		if err := <-r.done; err != nil {
			t.Errorf("the UI stopped with %v", err)
		}
	})
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && r.app == ""; time.Sleep(10 * time.Millisecond) {
		for line := range strings.Lines(r.logs.String()) {
			var record struct {
				Msg, Listen string
				ProbeListen string `json:"probe_listen"`
			}
			if json.Unmarshal([]byte(line), &record) == nil && record.Msg == "serving" {
				r.app, r.probes = "http://"+record.Listen, "http://"+record.ProbeListen
			}
		}
	}
	if r.app == "" {
		t.Fatalf("the UI wrote no serving record:\n%s", r.logs.String())
	}
	return r
}

// flags lists personal as an account and returns the flags that configure the UI with the database
// url names, the password in a mounted file, plain HTTP on an ephemeral loopback port, its probes on
// probes, and the configured consent redirect another than the default.
func flags(t *testing.T, url, probes string) []string {
	t.Helper()
	u, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(t.Context(), "INSERT INTO accounts (account_id, provider) VALUES ('personal', 'gmail') ON CONFLICT DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyFile, privateKeyFile, _ := writeKeys(t)
	return []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + u.User.Username(), "--database.sslmode=disable", "--database.password_file=" + passwordFile,
		"--listen=127.0.0.1:0", "--probe_listen=" + probes, "--credential.public_key_file=" + publicKeyFile,
		"--credential.private_key_files=[" + privateKeyFile + "]", "--consent_redirect=http://[::1]:5000/", "--stream_interval=50ms",
	}
}

// answer is one response the UI gave.
type answer struct {
	status int
	header http.Header
	body   string
}

// do sends one request to base and reads the whole answer, following no redirect.
func (r *running) do(method, base, path string, header http.Header, cookies ...*http.Cookie) answer {
	r.t.Helper()
	req, err := http.NewRequestWithContext(r.t.Context(), method, base+path, http.NoBody)
	if err != nil {
		r.t.Fatal(err)
	}
	for name, values := range header {
		req.Header[name] = values
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		r.t.Fatal(err)
	}
	return answer{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

var tokenMeta = regexp.MustCompile(`<meta name="mediated-mailbox.request_token" content="([^"]*)">`)

// Every endpoint the spec lists sits behind the checks. Each answer of each endpoint, the redirect at
// /, the entry document and the bundle under /ui/, and the read API's answers and errors under
// /ui/api/, carries the content security policy and its request's id and is logged by its line. A
// path under the root issues the session scoped to the root and / issues none. A state-changing
// request to any endpoint without its session's request token is refused with stale_page before the
// endpoint sees it, and the token the entry document carries lets it through. The entry document
// hands the browser the configured consent redirect. A path outside every endpoint's pattern is the
// process's own 404 (ADR-0061, ADR-0062, ADR-0137, ADR-0142, docs/UI.md sections 15 and 18.1).
func TestEveryEndpointSitsBehindTheChecks(t *testing.T) {
	r := start(t, postgres.URL(t))
	paths := []string{
		"/", "/ui/", "/ui/accounts/personal", "/ui/setup", "/ui/.gitkeep",
		"/ui/api/accounts", "/ui/api/accounts/personal/system", "/ui/api/accounts/all/system", "/ui/api/lens", "/ui/api/nothing",
	}
	if _, err := os.Stat(filepath.Join("..", "browser", "dist", "main.js")); err == nil {
		paths = append(paths, "/ui/main.js", "/ui/main.css")
	}
	for _, p := range paths {
		a := r.do(http.MethodGet, r.app, p, nil)
		if got := a.header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != policy {
			t.Errorf("%s answered %d with the policy %q, want exactly %q", p, a.status, got, policy)
		}
		id := a.header.Get("X-Request-Id")
		if id == "" || !strings.Contains(r.logs.String(), `"request_id":"`+id+`"`) {
			t.Errorf("%s answered %d with the request id %q, which no request line logs", p, a.status, id)
		}
		cookies := (&http.Response{Header: a.header}).Cookies()
		switch {
		case p == "/" && (a.status != http.StatusFound || a.header.Get("Location") != "/ui/" || len(cookies) != 0):
			t.Errorf("/ answered %d to %q with cookies %v, want a redirect to /ui/ and no session", a.status, a.header.Get("Location"), cookies)
		case p != "/" && (len(cookies) != 1 || cookies[0].Name != "ui_session" || cookies[0].Path != "/ui/"):
			t.Errorf("%s answered %d with cookies %v, want the session scoped to /ui/", p, a.status, cookies)
		}
	}
	entry := r.do(http.MethodGet, r.app, "/ui/setup", nil)
	if !strings.Contains(entry.body, `<meta name="mediated-mailbox.consent_redirect" content="http://[::1]:5000/">`) {
		t.Errorf("the entry document does not carry the configured redirect:\n%s", entry.body)
	}
	m := tokenMeta.FindStringSubmatch(entry.body)
	session := (&http.Response{Header: entry.header}).Cookies()
	if m == nil || len(session) != 1 {
		t.Fatalf("the entry document carried no token or no session:\n%s", entry.body)
	}
	for _, c := range []struct {
		path string
		// passed is what the endpoint answers once the checks let the request through.
		passed int
	}{{"/", http.StatusMethodNotAllowed}, {"/ui/setup", http.StatusMethodNotAllowed}, {"/ui/api/setup/connect", http.StatusBadRequest}} {
		if a := r.do(http.MethodPost, r.app, c.path, nil, session[0]); a.status != http.StatusForbidden || !strings.Contains(a.body, `"code":"stale_page"`) {
			t.Errorf("POST %s without the token answered %d %s, want 403 stale_page", c.path, a.status, a.body)
		}
		if a := r.do(http.MethodPost, r.app, c.path, http.Header{"X-Request-Token": {m[1]}}, session[0]); a.status != c.passed {
			t.Errorf("POST %s with the session's token answered %d %s, want %d from the endpoint", c.path, a.status, a.body, c.passed)
		}
	}
	for _, p := range []string{"/personal", "/setup", "/main.js", "/api/accounts", "/mcp"} {
		if a := r.do(http.MethodGet, r.app, p, nil); a.status != http.StatusNotFound {
			t.Errorf("%s answered %d, want %d, since the UI serves nothing outside its root but /", p, a.status, http.StatusNotFound)
		}
	}
}

// An open event stream ends once the process starts to stop, through the context Draining returns, so
// the process drains its other requests rather than waiting out an answer that never ends, and it
// stays open until then (ADR-0058, ADR-0142).
func TestAnOpenStreamEndsWhenTheProcessDrains(t *testing.T) {
	r := start(t, postgres.URL(t))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, r.app+"/ui/api/accounts/personal/events", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // the goroutine reading the stream closes it
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("the stream answered %d as %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	ended := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, resp.Body)
		ended <- errors.Join(err, resp.Body.Close())
	}()
	select {
	case err := <-ended:
		t.Fatalf("the stream ended before the process drained: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	r.stop()
	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("the stream ended with %v, want a clean end", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream was still open five seconds after the process began to stop")
	}
}

// The UI is ready only while the database answers a ping, and its readiness probe names the database
// when it does not (ADR-0142).
func TestReadinessFollowsTheDatabase(t *testing.T) {
	url := postgres.URL(t)
	r := start(t, url)
	awaitReadiness(r, http.StatusOK, "")
	open := shutOut(t, url)
	awaitReadiness(r, http.StatusServiceUnavailable, "not ready: database: the database does not answer a ping")
	open()
	awaitReadiness(r, http.StatusOK, "")
}

// awaitReadiness asks the readiness probe until it answers status with body in it, for up to the
// report's interval and its timeout together with a margin.
func awaitReadiness(r *running, status int, body string) {
	r.t.Helper()
	var got answer
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		got = r.do(http.MethodGet, r.probes, "/readyz", nil)
		if got.status == status && strings.Contains(got.body, body) {
			return
		}
	}
	r.t.Fatalf("the readiness probe answers %d %q, want %d with %q", got.status, got.body, status, body)
}

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

// The UI refuses to start on a schema missing its chain's newest migration, through the schema check
// database.Get runs on the read API endpoint's pool, before it serves (ADR-0140, ADR-0142).
func TestTheUIRefusesASchemaItCannotRunOn(t *testing.T) {
	dir := filepath.Join("..", "..", "db", "migrations")
	newest := int64(0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		var v int64
		if _, err := fmt.Sscanf(e.Name(), "%d_", &v); err == nil {
			newest = max(newest, v)
		}
	}
	behind := postgres.ChainBefore(t, dir, newest)
	u, err := neturl.Parse(behind.URL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyFile, privateKeyFile, _ := writeKeys(t)
	var logs lockedBuffer
	// A check that let the schema through would leave the UI serving, so a deadline ends the run.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = run(ctx, []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + u.User.Username(), "--database.sslmode=disable", "--database.password_file=" + passwordFile,
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0", "--credential.public_key_file=" + publicKeyFile,
		"--credential.private_key_files=[" + privateKeyFile + "]",
	}, nil, &logs, io.Discard)
	if !errors.Is(err, migrations.ErrNotMigrated) {
		t.Errorf("on a schema missing the chain's newest migration the run returned %v, want the start refused", err)
	}
	if strings.Contains(logs.String(), `"msg":"serving"`) {
		t.Errorf("the UI served on a schema it cannot run on:\n%s", logs.String())
	}
}

// A public key matching none of the private keys stops the UI's start before it serves, since the
// setups would seal credentials and client secrets to a key the client-secret package could not open
// with. The setups load and log the public key, and the client-secret package loads the private keys
// and refuses the pair, after the database is reached (ADR-0088, ADR-0092).
func TestAPublicKeyMatchingNoPrivateKeyStopsTheUIsStart(t *testing.T) {
	u, err := neturl.Parse(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyFile, _, keyID := writeKeys(t)
	_, otherPrivateKeyFile, _ := writeKeys(t)
	var logs lockedBuffer
	err = run(bounded(t), []string{
		"--database.host=" + u.Hostname(), "--database.port=" + u.Port(), "--database.name=" + strings.TrimPrefix(u.Path, "/"),
		"--database.user=" + u.User.Username(), "--database.sslmode=disable", "--database.password_file=" + passwordFile,
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0", "--credential.public_key_file=" + publicKeyFile,
		"--credential.private_key_files=[" + otherPrivateKeyFile + "]",
	}, nil, &logs, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "matches none of the private keys") {
		t.Errorf("with a public key matching none of the private keys the run returned %v, want the start refused", err)
	}
	if !strings.Contains(logs.String(), `"key_id":"`+keyID+`"`) {
		t.Errorf("the start did not log the public key it would seal to:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), `"msg":"serving"`) {
		t.Errorf("the UI served with a public key matching none of its private keys:\n%s", logs.String())
	}
}

// VERIFICATIONS' row for the log level, for the UI on its spec. Under log_level set to warn the UI
// writes every effective value with its source, serves and reports ready, and writes no record at info
// past the configuration, so the setups' record of the public key they seal to and the serving record
// are held back. With no log_level it writes both at info, so the run under warn holds back records
// the UI writes (ADR-0122).
func TestTheConfiguredLevelGovernsTheLog(t *testing.T) {
	url := postgres.URL(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	probes := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	var logs lockedBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	args := append(flags(t, url, probes), "--log_level=warn")
	go func() { done <- run(ctx, args, nil, &logs, io.Discard) }()
	// The serving record names the probes' address at info, which warn holds back, so the probes listen
	// where the flag puts them and the readiness probe is asked until it answers 200.
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the UI under warn did not report ready:\n%s", logs.String())
		}
		if ready(t, "http://"+probes) {
			break
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the UI under warn stopped with %v", err)
	}
	var sources []string
	for line := range strings.Lines(logs.String()) {
		var record struct{ Msg, Level, Path, Source string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("the log holds a line that is not a JSON record: %q", line)
		}
		switch {
		case record.Msg == "configuration":
			sources = append(sources, record.Path+" "+record.Source)
		case record.Level == "INFO" || record.Level == "DEBUG":
			t.Errorf("under warn the UI wrote %q at %s", record.Msg, record.Level)
		}
	}
	if !slices.Contains(sources, "log_level flag --log_level") || !slices.Contains(sources, "stream_interval flag --stream_interval") {
		t.Errorf("under warn the configuration written is %v, want every value with its source", sources)
	}

	loud := start(t, url)
	for _, msg := range []string{"sealing to the public key", "serving"} {
		if !strings.Contains(loud.logs.String(), `"level":"INFO","msg":"`+msg+`"`) {
			t.Errorf("with no log_level the UI wrote no %q record at info:\n%s", msg, loud.logs.String())
		}
	}
}

// ready reports whether the readiness probe at probes answers 200, false while nothing listens.
func ready(t *testing.T, probes string) bool {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, probes+"/readyz", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	return res.Body.Close() == nil && res.StatusCode == http.StatusOK
}

// The UI runs no job kind, so its pool's series carry the program's label alone and no job_kind
// (ADR-0125, ADR-0142).
func TestTheUIsPoolSeriesCarryNoJobKind(t *testing.T) {
	r := start(t, postgres.URL(t))
	metrics := r.do(http.MethodGet, r.probes, "/metrics", nil).body
	if !strings.Contains(metrics, "\n"+`mediated_mailbox_db_pool_max_connections{component="ui"} `) || strings.Contains(metrics, "job_kind") {
		t.Errorf("the UI's pool serves its series with a job_kind, or not at all:\n%s", metrics)
	}
}
