//go:build integration

package connections_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// role is the user the process connects as in these tests, the application user every program
// connects as (ADR-0143), so its grants apply as they do in production.
const role = postgres.ApplicationUser

// program is a configuration type composed as the API server's is for what the connections read, the
// process's parts and the accounts part inline, the database part and the credential part.
type program struct {
	config.Process    `yaml:",inline"`
	config.Operations `yaml:",inline"`
	config.Accounts   `yaml:",inline"`
	Database          config.Database `yaml:"database"`
	Credential        config.Keys     `yaml:"credential"`
}

func (p program) Check() error { return p.Database.Check() }

func (program) Defaults(int) program {
	return program{
		Process: config.DefaultProcess(), Operations: config.DefaultOperations(), Accounts: config.DefaultAccounts(),
		Database: config.DefaultDatabase(4),
	}
}

func (p program) DatabasePart() config.Database { return p.Database }

func (p program) KeysPart() config.Keys { return p.Credential }

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

// restoreOnCleanup runs a grant when the test ends, so a test that revokes one to make a statement
// fail leaves the role as the migration chain made it for the tests after it.
func restoreOnCleanup(t *testing.T, grant string) {
	t.Helper()
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := conn.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := conn.Exec(ctx, grant); err != nil {
			t.Errorf("%s: %v", grant, err)
		}
	})
}

// reset empties the tables the connections read. Every test of the package shares one database,
// and a load reads every account in it.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{"rate_grants", "rate_state", "account_state", "accounts", "oauth_clients"} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// keyPair is a generated key pair with its mounted files.
type keyPair struct {
	private     open.PrivateKey
	public      seal.PublicKey
	privateFile string
	publicFile  string
}

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
	private, err := open.ParsePrivateKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	k := keyPair{private: private, public: private.PublicKey(), privateFile: filepath.Join(dir, "private"), publicFile: filepath.Join(dir, "public")}
	if err := os.WriteFile(k.privateFile, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k.publicFile, key.PublicKey().Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return k
}

// keyring returns a keyring sealing to current and holding current's private key and those of old.
func keyring(t *testing.T, current keyPair, old ...keyPair) *open.Keyring {
	t.Helper()
	keys := []open.PrivateKey{current.private}
	for _, o := range old {
		keys = append(keys, o.private)
	}
	ring, err := open.NewKeyring(current.public, keys...)
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

func sealed(t *testing.T, to keyPair, plaintext string, c seal.Context) []byte {
	t.Helper()
	b, err := to.public.Seal([]byte(plaintext), c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// account writes an account and a state row holding credential, which may be nil.
func account(t *testing.T, conn *pgx.Conn, id, provider string, credential []byte) {
	t.Helper()
	listedOnly(t, conn, id, provider)
	must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", id, credential)
}

// listedOnly writes an account with no state row.
func listedOnly(t *testing.T, conn *pgx.Conn, id, provider string) {
	t.Helper()
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, $2)", id, provider)
}

// client writes an OAuth client of a provider under its name, its secret as given.
func client(t *testing.T, conn *pgx.Conn, name, provider, id string, secret []byte) {
	t.Helper()
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, $2, $3, $4)", name, provider, id, secret)
}

// through makes an account connect through the named client, as account setup writes it.
func through(t *testing.T, conn *pgx.Conn, account, client string) {
	t.Helper()
	must(t, conn, "UPDATE accounts SET oauth_client = $2 WHERE account_id = $1", account, client)
}

// storedCredential reads an account's stored credential as the superuser.
func storedCredential(t *testing.T, conn *pgx.Conn, id string) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT credential FROM account_state WHERE account_id = $1", id).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// gmail and fastmail are the providers the tests' adapters serve, one whose accounts connect through
// an OAuth client and one whose accounts need none.
var (
	gmail    = fake.Provider{Name: "gmail", UsesClient: true}
	fastmail = fake.Provider{Name: "fastmail"}
)

// refusedPrefix marks a credential the tests' provider refuses, as a provider refuses a revoked grant.
const refusedPrefix = "refused"

// provided is the provider behind the tests' adapters, one Fake per account and the tokens each
// mailbox was built over, newest last.
type provided struct {
	mu     sync.Mutex
	fakes  map[string]*fake.Fake
	tokens map[string][]*fake.Token
}

// mailbox is the tests' adapters' mailbox for an account over token. A credential starting with
// refusedPrefix is refused, as the provider refuses a revoked grant.
func (p *provided) mailbox(account string, token *fake.Token) (*fake.Throttled, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.fakes[account]
	if !ok {
		var err error
		if f, err = fake.New(fake.Config{Account: account}); err != nil {
			return nil, err
		}
		p.fakes[account] = f
	}
	p.tokens[account] = append(p.tokens[account], token)
	return fake.Throttle(f, func(fake.Call) error {
		if strings.HasPrefix(string(token.Credential()), refusedPrefix) {
			return coremail.ErrAuthentication
		}
		return nil
	}, time.Now), nil
}

// latest returns the token the account's latest mailbox was built over, nil for none.
func (p *provided) latest(account string) *fake.Token {
	p.mu.Lock()
	defer p.mu.Unlock()
	tokens := p.tokens[account]
	if len(tokens) == 0 {
		return nil
	}
	return tokens[len(tokens)-1]
}

// built returns how many mailboxes were built for the account.
func (p *provided) built(account string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.tokens[account])
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

// records returns every log record of the level, each its message and its account and client
// attributes.
func (b *lockedBuffer) records(t *testing.T, level string) []map[string]string {
	t.Helper()
	var out []map[string]string
	for line := range strings.Lines(b.String()) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["level"] != level {
			continue
		}
		r := map[string]string{}
		for _, k := range []string{"msg", "account", "client"} {
			if v, ok := rec[k].(string); ok {
				r[k] = v
			}
		}
		out = append(out, r)
	}
	return out
}

// running is a process a test runs, with what its one job kind's Build got.
type running struct {
	// other is the caller of a second job kind, as each job kind of a process builds one.
	other       *connections.Caller
	session     *sessions.Session
	connections *connections.Connections
	caller      *connections.Caller
	provider    *provided
	logs        *lockedBuffer
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
		"--probe_listen=127.0.0.1:0", "--account_reload_interval=1h",
	}
}

// start runs a process whose one job kind gets the Session, the connections over the tests'
// adapters, and a caller over its own pool, with current's key pair the one credentials are sealed to
// and the private keys of old beside it. The process stops when the test ends. The Session's first
// reload runs once the process serves, and start returns once it has run.
func start(t *testing.T, current keyPair, old ...keyPair) *running {
	t.Helper()
	privates := []string{current.privateFile}
	for _, o := range old {
		privates = append(privates, o.privateFile)
	}
	args := append(applicationFlags(t),
		"--credential.public_key_file="+current.publicFile, "--credential.private_key_files=["+strings.Join(privates, ", ")+"]")
	r := &running{provider: &provided{fakes: map[string]*fake.Fake{}, tokens: map[string][]*fake.Token{}}, logs: &lockedBuffer{}}
	list := fake.Adapters(r.provider.mailbox, gmail, fastmail)
	built := make(chan error, 2)
	spec := process.Spec[program]{Name: "test", Jobs: []process.Job[program]{{
		Name: "other",
		Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			err := func() error {
				s, err := sessions.Get(ctx, p, c)
				if err != nil {
					return err
				}
				conns, err := connections.Of(ctx, p, c, s)
				if err != nil {
					return err
				}
				pool, err := database.Get(ctx, p, c)
				if err != nil {
					return err
				}
				r.other, err = conns.Caller(p, pool, provider.Options{Class: provider.Sync, Timeout: time.Minute})
				return err
			}()
			built <- err
			return idle(), err
		},
	}, {
		Name: "work",
		Build: func(ctx context.Context, p process.Capabilities, c program) (schedule.Plan, error) {
			err := func() error {
				s, err := sessions.Get(ctx, p, c)
				if err != nil {
					return err
				}
				conns, err := connections.Of(ctx, p, c, s)
				if err != nil {
					return err
				}
				pool, err := database.Get(ctx, p, c)
				if err != nil {
					return err
				}
				caller, err := conns.Caller(p, pool, provider.Options{Class: provider.Batch, Timeout: time.Minute})
				if err != nil {
					return err
				}
				r.session, r.connections, r.caller = s, conns, caller
				return nil
			}()
			built <- err
			return idle(), err
		},
	}}}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- process.RunContext(ctx, args, nil, r.logs, r.logs, spec, connections.WithAdapters(list))
	}()
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
			t.Fatalf("the process ended before its build: %v\n%s", err, r.logs.String())
		}
	}
	// The Session's first reload runs once the process serves, and the process is ready once it has
	// loaded everything, so what a test changes after start is read by the reloads the test asks for.
	waitReady(t, r.logs)
	return r
}

// waitReady waits until the running process's readiness probe, whose address the serving record
// names, answers ready.
func waitReady(t *testing.T, logs *lockedBuffer) {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		addr := probeAddress(logs.String())
		if addr == "" {
			continue
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/readyz", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode == http.StatusOK {
			return
		}
	}
	t.Fatalf("the process was not ready within twenty seconds:\n%s", logs.String())
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

// idle is a plan that lists nothing.
func idle() schedule.Plan {
	return schedule.Plan{
		Keys: func(context.Context) ([]string, error) { return nil, nil },
		Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
	}
}

// reload reloads the Session and fails the test on an error.
func (r *running) reload(t *testing.T) {
	t.Helper()
	if err := r.session.Reload(t.Context()); err != nil {
		t.Fatalf("reloading: %v", err)
	}
}

// begin begins a unit of work on the account and lists its labels, which builds its mailbox over the
// account's token, and returns the unit.
func (r *running) begin(t *testing.T, account string) *connections.Work {
	t.Helper()
	w, err := r.connections.Begin(account, r.caller, time.Time{})
	if err != nil {
		t.Fatalf("beginning a unit of work on %s: %v", account, err)
	}
	return w
}

// token returns what the token the account's latest unit of work began over holds, its client's
// identifier and secret and its credential, or "not connected" when no unit of work begins on it.
func (r *running) token(t *testing.T, account string) string {
	t.Helper()
	w, err := r.connections.Begin(account, r.caller, time.Time{})
	if errors.Is(err, connections.ErrNotConnected) {
		return "not connected"
	}
	if err != nil {
		t.Fatalf("beginning a unit of work on %s: %v", account, err)
	}
	if _, err := w.Mailbox().ListLabels(t.Context()); err != nil && !errors.Is(err, coremail.ErrAuthentication) {
		t.Fatalf("listing the labels of %s: %v", account, err)
	}
	tok := r.provider.latest(account)
	c := tok.Client()
	if c == (coremail.OAuthClient{}) {
		return string(tok.Credential())
	}
	return fmt.Sprintf("%s through %s %s", tok.Credential(), c.ID, c.Secret)
}

// tokens returns token for each account the Session lists.
func (r *running) tokens(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, a := range r.session.Accounts() {
		out[a.ID] = r.token(t, a.ID)
	}
	return out
}

// forget removes an account and everything stored for it, as removing an account does.
func forget(t *testing.T, conn *pgx.Conn, id string) {
	t.Helper()
	for _, table := range []string{"rate_grants", "rate_state", "account_state", "accounts"} {
		must(t, conn, "DELETE FROM "+table+" WHERE account_id = $1", id)
	}
}
