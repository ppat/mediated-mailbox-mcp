//go:build integration

package accountload_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// The role the loader connects as in these tests. Delta sync's role reads and writes everything the
// library's statements reach, as the mediator's, backfill's and the reorg workload's do (ADR-0075).
const role = "mediated_mailbox_sync"

// pool returns a pool connecting as role, so row-level security and the role's grants apply as they
// do in production.
func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// superuser returns a connection that bypasses row-level security, to write what the UI's setups and
// the operator would write, and to break what the tests break.
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

// restoreOnCleanup runs a grant when the test ends, so a test that revokes one to make a statement fail
// leaves the role as the migration chain made it for the tests after it.
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

// reset empties the tables the loader reads. Every test of the package shares one database, and a
// load reads every account in it.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	must(t, conn, "DELETE FROM account_state")
	must(t, conn, "DELETE FROM accounts")
	must(t, conn, "DELETE FROM oauth_clients")
}

// keyPair is a generated key pair.
type keyPair struct {
	private open.PrivateKey
	public  seal.PublicKey
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
	return keyPair{private: private, public: private.PublicKey()}
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

// client writes a provider's OAuth client, its secret sealed to the key pair.
func client(t *testing.T, conn *pgx.Conn, provider, id string, secret []byte) {
	t.Helper()
	must(t, conn, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ($1, $2, $3)", provider, id, secret)
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

// logBuffer collects a loader's log lines.
type logBuffer struct{ bytes.Buffer }

func (b *logBuffer) logger() *slog.Logger { return slog.New(slog.NewJSONHandler(b, nil)) }

// errors returns the message of every error-level record with its account and provider attributes.
func (b *logBuffer) errors(t *testing.T) []map[string]string {
	t.Helper()
	var out []map[string]string
	for line := range strings.Lines(b.String()) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if rec["level"] != "ERROR" {
			continue
		}
		r := map[string]string{}
		for _, k := range []string{"msg", "account", "provider"} {
			if v, ok := rec[k].(string); ok {
				r[k] = v
			}
		}
		out = append(out, r)
	}
	return out
}

// load builds a loader over a pool as role and loads it, failing the test on an error.
func load(t *testing.T, ring *open.Keyring, log *logBuffer) *accountload.Loader {
	t.Helper()
	l := accountload.New(pool(t), ring, log.logger())
	if err := l.Load(t.Context()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return l
}

// credentials returns each served account's opened credential, "not connected" for one without.
func credentials(s *accountload.Snapshot) map[string]string {
	out := map[string]string{}
	for _, a := range s.Accounts() {
		if a.Connected() {
			out[a.ID()] = string(a.Credential())
		} else {
			out[a.ID()] = "not connected"
		}
	}
	return out
}
