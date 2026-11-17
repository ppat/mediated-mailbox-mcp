//go:build integration

package app_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/api/app"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// superuser returns a connection that holds every privilege, to write the rows the tests stage and
// to break what the tests break.
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

// reset empties the tables that refer to accounts and the accounts themselves. Every test of the
// package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{"reorg_plans", "messages", "audit_log", "account_state", "rate_grants", "rate_state", "policy_rules", "accounts", "oauth_clients"} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// revoke revokes a grant of the application user until grantBack or the end of the test.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+postgres.ApplicationUser)
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
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+postgres.ApplicationUser); err != nil {
			t.Error(err)
		}
	})
}

// grantBack restores a grant revoke took.
func grantBack(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "GRANT "+privilege+" TO "+postgres.ApplicationUser)
}

// newAccounts creates two accounts named for the test.
func newAccounts(t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	prefix := "acct-" + strings.ToLower(rand.Text()[:8])
	accounts := []string{prefix + "-a", prefix + "-b"}
	for _, account := range accounts {
		must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	}
	return accounts
}

// gmail is the provider the tests' accounts name, served in the tests by an adapter over the provider
// fake that connects through an OAuth client, as Gmail's does.
var gmail = fake.Provider{Name: "gmail", UsesClient: true}

// household is the name of the client the tests' accounts connect through. It is not the provider's
// name, so nothing finds it by the provider.
const household = "household"

// keyPair is a generated key pair credentials are sealed to, with the files the API server loads it
// from.
type keyPair struct {
	public                  seal.PublicKey
	publicFile, privateFile string
}

// newKeyPair generates a key pair and writes its files.
func newKeyPair(t *testing.T) keyPair {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	k := keyPair{publicFile: filepath.Join(dir, "public"), privateFile: filepath.Join(dir, "private")}
	if err := os.WriteFile(k.privateFile, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k.publicFile, key.PublicKey().Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if k.public, err = seal.ParsePublicKey(key.PublicKey().Bytes()); err != nil {
		t.Fatal(err)
	}
	return k
}

// connect writes an account's state row holding its credential, account-token, sealed to k.
func connect(t *testing.T, conn *pgx.Conn, k keyPair, account string) {
	t.Helper()
	credential, err := k.public.Seal([]byte(account+"-token"), seal.AccountCredential(account))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", account, credential)
}

// store replaces an account's stored credential with credential sealed to k.
func store(t *testing.T, conn *pgx.Conn, k keyPair, account, credential string) {
	t.Helper()
	sealed, err := k.public.Seal([]byte(credential), seal.AccountCredential(account))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "UPDATE account_state SET credential = $2 WHERE account_id = $1", account, sealed)
}

// client writes a client of the tests' provider under its name, its secret sealed to k.
func client(t *testing.T, conn *pgx.Conn, k keyPair, name, id, secret string) {
	t.Helper()
	sealed, err := k.public.Seal([]byte(secret), seal.ClientSecret(name))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, $2, $3, $4)", name, gmail.Name, id, sealed)
}

// through makes an account connect through the named client, as account setup writes it.
func through(t *testing.T, conn *pgx.Conn, account, client string) {
	t.Helper()
	must(t, conn, "UPDATE accounts SET oauth_client = $2 WHERE account_id = $1", account, client)
}

// logBuffer collects what the API server's process writes to standard output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// count returns how many records carry the message msg.
func (b *logBuffer) count(msg string) int {
	return strings.Count(b.String(), `"msg":"`+msg+`"`)
}

// databaseFlags returns the flags naming the test database, connected to as the application user.
// pgrun gives the user the superuser's password, so the password file holds the one postgres.URL
// carries.
func databaseFlags(t *testing.T) []string {
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
		"--database.user=" + postgres.ApplicationUser, "--database.sslmode=disable", "--database.password_file=" + path,
	}
}

// bearer is the bearer token every test's API server admits.
const bearer = "s3cret"

// tokenFile writes the bearer token to a file standing in for the mounted token and returns its path.
func tokenFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(bearer), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// apiFlags returns the flags of an API server over the test database whose credentials are sealed to k,
// serving plain HTTP and its probes on loopback ports the system picks, reloading on interval, with
// each provider call bounded by timeout, with each flag of more in place of the one of its name.
func apiFlags(t *testing.T, k keyPair, interval, timeout time.Duration, more ...string) []string {
	t.Helper()
	out := append(databaseFlags(t),
		"--token_file="+tokenFile(t), "--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0",
		"--credential.public_key_file="+k.publicFile, "--credential.private_key_files=["+k.privateFile+"]",
		"--account_reload_interval="+interval.String(), "--provider_timeout="+timeout.String())
	for _, m := range more {
		name, _, _ := strings.Cut(m, "=")
		out = slices.DeleteFunc(out, func(a string) bool { return strings.HasPrefix(a, name+"=") })
		out = append(out, m)
	}
	return out
}

// scannerRevision returns the revision of the scanner part that the configuration library computes for
// a start with args, the one every serve-time check under it records.
func scannerRevision(t *testing.T, args []string) string {
	t.Helper()
	got, err := settings.Load(config.DefaultAPI(runtime.NumCPU()), args, nil)
	if err != nil {
		t.Fatal(err)
	}
	return got.Revisions["scanner"]
}

// server is a running API server, started through its spec as its main.go starts it, with the
// addresses its serving record names and what it wrote.
type server struct {
	app, probes string
	logs        *logBuffer
	stop        func()
}

// start runs the API server's spec with args until the test ends or stop is called, its connections
// making every token and mailbox from list. It returns once the process serves. A start that stops
// before it serves fails the test.
func start(t *testing.T, args []string, list process.Option) *server {
	t.Helper()
	s := &server{logs: &logBuffer{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- process.RunContext(ctx, args, nil, s.logs, io.Discard, app.Spec(), list) }()
	var once sync.Once
	s.stop = func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("the API server ended with %v", err)
				}
			case <-time.After(20 * time.Second):
				t.Error("the API server did not stop")
			}
		})
	}
	t.Cleanup(s.stop)
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("the API server stopped before it served: %v\n%s", err, s.logs.String())
		default:
		}
		for line := range strings.Lines(s.logs.String()) {
			var rec struct {
				Msg         string `json:"msg"`
				Listen      string `json:"listen"`
				ProbeListen string `json:"probe_listen"`
			}
			if err := json.Unmarshal([]byte(line), &rec); err == nil && rec.Msg == "serving" {
				s.app, s.probes = rec.Listen, rec.ProbeListen
				return s
			}
		}
	}
	t.Fatalf("the API server wrote no serving record:\n%s", s.logs.String())
	return nil
}

// ready returns the readiness probe's status and body.
func (s *server) ready(t *testing.T) (int, string) {
	t.Helper()
	status, body, _ := s.get(t, "http://"+s.probes+"/readyz", "")
	return status, body
}

// awaitReady waits until the readiness probe answers ready, failing the test after within.
func (s *server) awaitReady(t *testing.T, within time.Duration) {
	t.Helper()
	var body string
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var status int
		if status, body = s.ready(t); status == http.StatusOK {
			return
		}
	}
	t.Fatalf("the API server was not ready within %v: %s\n%s", within, body, s.logs.String())
}

// get sends a GET to url, with authorization when it is not empty, and returns the status, the body
// and the headers.
func (s *server) get(t *testing.T, url, authorization string) (int, string, http.Header) {
	t.Helper()
	return s.send(t, http.MethodGet, url, authorization, "")
}

// send sends one request and returns the status, the body and the headers.
func (s *server) send(t *testing.T, method, url, authorization, body string) (int, string, http.Header) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(out), resp.Header
}

// api calls an operation on the API root at path with the bearer token, and returns the status and
// the body.
func (s *server) api(t *testing.T, path string) (int, string) {
	t.Helper()
	status, body, _ := s.get(t, "http://"+s.app+path, "Bearer "+bearer)
	return status, body
}

// mcp calls a tool on the MCP root with the bearer token and returns the HTTP status and the tool's
// result: whether it is an error and its structured content.
func (s *server) mcp(t *testing.T, tool string, arguments map[string]string) (int, bool, string) {
	t.Helper()
	args, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + string(args) + `}}`
	status, body, _ := s.send(t, http.MethodPost, "http://"+s.app+"/mcp", "Bearer "+bearer, call)
	if i := strings.Index(body, "{"); i > 0 {
		body = strings.TrimSpace(body[i:])
	}
	var r struct {
		Result struct {
			IsError           bool            `json:"isError"`
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("the MCP root answered %d %s: %v", status, body, err)
	}
	return status, r.Result.IsError, string(r.Result.StructuredContent)
}

// served reports whether the API server serves the account, whose system status it then answers.
func (s *server) served(t *testing.T, account string) bool {
	t.Helper()
	status, _ := s.api(t, "/api/accounts/"+account+"/status")
	return status == http.StatusOK
}

// awaitServed waits until the API server serves the account, or no longer does, failing the test
// after ten seconds.
func (s *server) awaitServed(t *testing.T, account string, want bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if s.served(t, account) == want {
			return
		}
	}
	t.Fatalf("the API server's serving of %s did not become %v:\n%s", account, want, s.logs.String())
}

// reloaded waits until two reloads whose accounts and connections loaded have run since it was
// called, so what a test changed before the call is what the second read. It fails the test after
// ten seconds.
func (s *server) reloaded(t *testing.T) {
	t.Helper()
	from := s.logs.count("accounts served")
	for deadline := time.Now().Add(10 * time.Second); s.logs.count("accounts served") < from+2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no reload ran:\n%s", s.logs.String())
		}
	}
}

// metric returns the value of the series name whose labels are exactly labels on the metrics
// endpoint, a histogram's sample count for a histogram's name, and 0 when it serves none.
func (s *server) metric(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	samples := s.series(t, name)
	if v, ok := samples[labelText(labels)]; ok {
		return v
	}
	if v, ok := s.series(t, name+"_count")[labelText(labels)]; ok {
		return v
	}
	return 0
}

// series returns every sample of the series name on the metrics endpoint, keyed by its labels written
// as labelText writes them.
func (s *server) series(t *testing.T, name string) map[string]float64 {
	t.Helper()
	status, body, _ := s.get(t, "http://"+s.probes+"/metrics", "")
	if status != http.StatusOK {
		t.Fatalf("the metrics endpoint answered %d", status)
	}
	out := map[string]float64{}
	for line := range strings.Lines(body) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		series, labels, _ := strings.Cut(key, "{")
		if series != name {
			continue
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Fatalf("the sample %q: %v", line, err)
		}
		out[strings.TrimSuffix(labels, "}")] = v
	}
	return out
}

// labelText writes labels as the metrics endpoint writes them, in name order.
func labelText(labels map[string]string) string {
	names := make([]string, 0, len(labels))
	for n := range labels {
		names = append(names, n)
	}
	slices.Sort(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%q", n, labels[n]))
	}
	return strings.Join(parts, ",")
}
