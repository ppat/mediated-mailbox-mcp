//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	providermail "github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/app"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// role is the user the worker connects as in these tests, the application user every program
// connects as (ADR-0143), so its grants apply as they do in production.
const role = postgres.ApplicationUser

// household is the OAuth client every account of these tests connects through, as account setup
// writes it.
const household = "household"

// gmail is the provider the tests' adapters serve, whose accounts connect through an OAuth client as
// Gmail's do (ADR-0106). The adapters are the provider fake's, so nothing reaches Google (ADR-0043).
var gmail = fake.Provider{Name: "gmail", UsesClient: true}

// keyPair is a generated key pair with its mounted files, the private key's seed in one and the public
// key in the other, as the command-line program's keygen writes them.
type keyPair struct {
	public      seal.PublicKey
	privateFile string
	publicFile  string
}

// generate writes a new key pair's files into a directory of the test's own.
func generate(t *testing.T) keyPair {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	public, err := seal.ParsePublicKey(key.PublicKey().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	k := keyPair{public: public, privateFile: filepath.Join(dir, "private"), publicFile: filepath.Join(dir, "public")}
	if err := os.WriteFile(k.privateFile, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k.publicFile, key.PublicKey().Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return k
}

// sealed returns plaintext sealed to k under c.
func sealed(t *testing.T, k keyPair, plaintext string, c seal.Context) []byte {
	t.Helper()
	b, err := k.public.Seal([]byte(plaintext), c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

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

// revoke revokes a grant of the application user, and grants it again when the test ends, so the
// tests after it see the user as the migration chain made it.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+role)
	t.Cleanup(func() { restore(t, "GRANT "+privilege+" TO "+role) })
}

// restore runs a statement on a connection of its own as the test ends.
func restore(t *testing.T, statement string) {
	ctx := context.Background()
	c, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		if err := c.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	if _, err := c.Exec(ctx, statement); err != nil {
		t.Errorf("%s: %v", statement, err)
	}
}

// lockedBuffer collects what the process writes from several goroutines.
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

// logRecord is one log record's level, message and the attributes the tests read.
type logRecord struct {
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Account string `json:"account"`
	JobKind string `json:"job_kind"`
	Error   string `json:"error"`
	Path    string `json:"path"`
	Source  string `json:"source"`
}

// world is the database, the key pair and the provider behind one worker a test runs through its spec
// and process.RunContext, as its main.go runs it, apart from the provider fake the connections are
// handed through their one option for a test (ADR-0142).
type world struct {
	t    *testing.T
	conn *pgx.Conn
	keys keyPair
	logs *lockedBuffer

	mu sync.Mutex
	// ports are each account's mailbox behind the fake's adapter, a Fake with no mail unless the test
	// set one, and schedules what each account's calls are refused with, none unless set.
	ports     map[string]adapterMailbox
	schedules map[string]fake.Schedule
	// tokens are the tokens each account's mailboxes were built over, newest last.
	tokens map[string][]*fake.Token

	// privates are the private key files the worker loads, the world's own when none is set.
	privates []string

	probes string
	stop   func()
	done   chan error
}

// newWorld empties every table the worker reads and writes, generates the key pair credentials are
// sealed to, and stores the household client with its secret sealed to it.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		t: t, conn: superuser(t), keys: generate(t), logs: &lockedBuffer{},
		ports: map[string]adapterMailbox{}, schedules: map[string]fake.Schedule{}, tokens: map[string][]*fake.Token{},
	}
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"policy_rules", "rate_grants", "rate_state", "account_state", "accounts", "oauth_clients",
	} {
		must(t, w.conn, "DELETE FROM "+table)
	}
	must(t, w.conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, 'gmail', 'household-id', $2)",
		household, sealed(t, w.keys, "household-secret", seal.ClientSecret(household)))
	return w
}

// connect stores an account connected through the household client, holding credential sealed to the
// world's key, with both of its backfill passes ended when ended is set.
func (w *world) connect(account, credential string, ended bool) {
	w.t.Helper()
	must(w.t, w.conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ($1, 'gmail', $2)", account, household)
	must(w.t, w.conn, "INSERT INTO account_state (account_id, credential, backfill_pass1_complete, backfill_pass2_complete) VALUES ($1, $2, $3, $3)",
		account, sealed(w.t, w.keys, credential, seal.AccountCredential(account)), ended)
}

// adapterMailbox is an account's mailbox as the fake's adapter builds it, the Provider Port's operations
// and the rate profile the adapter declares for them, which fake.Throttle wraps.
type adapterMailbox interface {
	providermail.Mailbox
	RateProfile() mail.RateLimitProfile[context.Context]
}

// port sets the account's mailbox behind the fake's adapter.
func (w *world) port(account string, p adapterMailbox) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ports[account] = p
}

// schedule sets what the account's calls are refused with.
func (w *world) schedule(account string, s fake.Schedule) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.schedules[account] = s
}

// mailbox is the fake's adapters' mailbox for an account over token, the account's port behind its
// schedule.
func (w *world) mailbox(account string, token *fake.Token) (*fake.Throttled, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.ports[account]
	if !ok {
		f, err := fake.New(fake.Config{Account: account})
		if err != nil {
			return nil, err
		}
		p, w.ports[account] = f, f
	}
	w.tokens[account] = append(w.tokens[account], token)
	return fake.Throttle(p, func(c fake.Call) error {
		w.mu.Lock()
		s := w.schedules[account]
		w.mu.Unlock()
		if s == nil {
			return nil
		}
		return s(c)
	}, time.Now), nil
}

// built returns the tokens the account's mailboxes were built over, oldest first.
func (w *world) built(account string) []*fake.Token {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.tokens[account])
}

// flags returns the flags naming the test database, connected to as the application user, the
// world's key files and a probe address of the system's choosing, each of more in place of the flag of
// its name or after them. pgrun gives the user the superuser's password, so the password file holds
// the one postgres.URL carries.
func (w *world) flags(more ...string) []string {
	w.t.Helper()
	privates := w.privates
	if privates == nil {
		privates = []string{w.keys.privateFile}
	}
	out := append(databaseFlags(w.t, postgres.URL(w.t)),
		"--credential.public_key_file="+w.keys.publicFile, "--credential.private_key_files=["+strings.Join(privates, ", ")+"]",
		"--probe_listen=127.0.0.1:0")
	for _, m := range more {
		name, _, _ := strings.Cut(m, "=")
		out = slices.DeleteFunc(out, func(a string) bool { return strings.HasPrefix(a, name+"=") })
		out = append(out, m)
	}
	return out
}

// databaseFlags returns the flags naming the database url names, connected to as the application
// user with url's password in a mounted file. pgrun gives the user the superuser's password.
func databaseFlags(t *testing.T, url string) []string {
	t.Helper()
	u, err := neturl.Parse(url)
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
	}
}

// start runs the worker through its spec with args after the world's flags, and returns once it
// serves its probes. The worker stops when the test ends, and a run that ends in an error fails the
// test.
func (w *world) start(args ...string) {
	w.t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	w.stop, w.done = stop, make(chan error, 1)
	list := fake.Adapters(w.mailbox, gmail)
	go func() {
		w.done <- process.RunContext(ctx, w.flags(args...), nil, w.logs, io.Discard, app.Spec(), connections.WithAdapters(list))
	}()
	w.t.Cleanup(w.halt)
	for deadline := time.Now().Add(20 * time.Second); w.probes == ""; time.Sleep(20 * time.Millisecond) {
		w.probes = probeAddress(w.logs.String())
		select {
		case err := <-w.done:
			w.done <- err
			w.t.Fatalf("the worker ended before it served: %v\n%s", err, w.logs.String())
		default:
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("the worker did not serve within twenty seconds:\n%s", w.logs.String())
		}
	}
}

// halt stops the worker and waits for its run, failing the test on its error. A second call does
// nothing.
func (w *world) halt() {
	if w.stop == nil {
		return
	}
	w.stop()
	w.stop = nil
	if err := <-w.done; err != nil {
		w.t.Errorf("the worker ended with %v", err)
	}
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

// probe returns the worker's probes' answer on path.
func (w *world) probe(path string) (int, string) {
	w.t.Helper()
	req, err := http.NewRequestWithContext(w.t.Context(), http.MethodGet, "http://"+w.probes+path, http.NoBody)
	if err != nil {
		w.t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	if err := errors.Join(err, res.Body.Close()); err != nil {
		w.t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

// eventually waits up to within for done to hold, failing the test naming what with the worker's logs
// when it does not.
func (w *world) eventually(within time.Duration, what string, done func() bool) {
	w.t.Helper()
	for deadline := time.Now().Add(within); !done(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			w.t.Fatalf("%s did not happen within %s:\n%s", what, within, w.logs.String())
		}
	}
}

// ready waits for the readiness probe to answer 200.
func (w *world) ready() {
	w.t.Helper()
	w.eventually(20*time.Second, "the worker reporting ready", func() bool {
		code, _ := w.probe("/readyz")
		return code == http.StatusOK
	})
}

// served returns whether the worker's metrics endpoint serves every line of want. A line with no
// value matches a series of that name and labels whatever its value.
func (w *world) served(want ...string) bool {
	w.t.Helper()
	return len(w.missing(want...)) == 0
}

// missing returns the lines of want the worker's metrics endpoint does not serve.
func (w *world) missing(want ...string) []string {
	w.t.Helper()
	code, body := w.probe("/metrics")
	if code != http.StatusOK {
		w.t.Fatalf("/metrics answered %d", code)
	}
	var out []string
	for _, line := range want {
		if !strings.Contains(body, "\n"+line+"\n") && (strings.Contains(line, " ") || !strings.Contains(body, "\n"+line+" ")) {
			out = append(out, line)
		}
	}
	return out
}

// scrape waits up to twenty seconds for the worker's metrics endpoint to serve every line of want,
// and fails the test for each line it does not.
func (w *world) scrape(want ...string) {
	w.t.Helper()
	for deadline := time.Now().Add(20 * time.Second); !w.served(want...); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			_, body := w.probe("/metrics")
			for _, line := range w.missing(want...) {
				w.t.Errorf("/metrics does not serve %s", line)
			}
			w.t.Logf("/metrics served:\n%s", body)
			return
		}
	}
}

// records returns every log record the worker wrote at level, "" for every level.
func (w *world) records(level string) []logRecord {
	w.t.Helper()
	var out []logRecord
	for line := range strings.Lines(w.logs.String()) {
		var r logRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		if level == "" || r.Level == level {
			out = append(out, r)
		}
	}
	return out
}

// jobs returns the job kinds holding a job for the account, sorted, read from the series of each job's
// latest success the scheduler serves for every job it holds.
func (w *world) jobs(account string) []string {
	w.t.Helper()
	_, body := w.probe("/metrics")
	var kinds []string
	for line := range strings.Lines(body) {
		if !strings.HasPrefix(line, "mediated_mailbox_job_last_success_timestamp_seconds{") || !strings.Contains(line, `account="`+account+`"`) {
			continue
		}
		_, rest, _ := strings.Cut(line, `job_kind="`)
		kind, _, _ := strings.Cut(rest, `"`)
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	return kinds
}

// runWorkerContext runs the worker through its spec as its main.go does, with the shipped adapters,
// until ctx ends, writing its records to logs.
func runWorkerContext(ctx context.Context, args, environ []string, logs io.Writer) error {
	return process.RunContext(ctx, args, environ, logs, io.Discard, app.Spec())
}

// runWorker runs the worker through its spec for a start meant to fail before it serves, and returns
// the start's error, failing the test when it serves for twenty seconds instead.
func runWorker(t *testing.T, args, environ []string, logs io.Writer) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	err := runWorkerContext(ctx, args, environ, logs)
	if ctx.Err() != nil {
		t.Fatalf("the start did not fail within twenty seconds, it returned %v", err)
	}
	return err
}
