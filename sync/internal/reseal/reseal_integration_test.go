//go:build integration

package reseal_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// syncPool returns a pool connecting as delta sync's role, so its grants apply as in production.
func syncPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=mediated_mailbox_sync"
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

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

func keyPair(t *testing.T) (open.PrivateKey, seal.PublicKey) {
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
	return private, private.PublicKey()
}

func sealed(t *testing.T, to seal.PublicKey, client, plaintext string) []byte {
	t.Helper()
	b, err := to.Seal([]byte(plaintext), seal.ClientSecret(client))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func stored(t *testing.T, conn *pgx.Conn, client string) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE name = $1", client).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// secrets returns the opened secret of each served account's client, "no client" for one without.
func secrets(s *accountload.Snapshot) map[string]string {
	out := map[string]string{}
	for _, a := range s.Accounts() {
		if c, ok := a.Client(); ok {
			out[a.ID()] = c.Name() + " " + string(c.Secret())
		} else {
			out[a.ID()] = "no client"
		}
	}
	return out
}

// D4's part of VERIFICATIONS' row for a write of a sealed value based on bytes since replaced, delta
// sync's re-seal of a client secret. Two clients of one provider are on an old key, and the operator
// replaces one of them after delta sync loaded it. The re-seal does not overwrite it. The stored
// secret stays the operator's, delta sync reads it again and holds it from then on for the account
// connecting through it, and its scan entry follows the stored bytes. The other client, which nobody
// replaced, is re-sealed to the current key under its own name, and its entry reads false (ADR-0089,
// ADR-0092, ADR-0106).
func TestAReSealNeverPutsBackAReplacedSecret(t *testing.T) {
	conn := superuser(t)
	oldPrivate, oldPublic := keyPair(t)
	newPrivate, newPublic := keyPair(t)
	ring, err := open.NewKeyring(newPublic, oldPrivate, newPrivate)
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', $1), ('employer', 'gmail', 'employer-id', $2)",
		sealed(t, oldPublic, "household", "first-secret"), sealed(t, oldPublic, "employer", "employer-secret"))
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('personal', 'gmail', 'household'), ('work', 'gmail', 'employer')")
	loader := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler), []string{"gmail"})
	if err := loader.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	replaced := sealed(t, oldPublic, "household", "operator-secret")
	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", replaced)

	scan := loader.ResealClients(t.Context(), reseal.Writer(syncPool(t)))

	if got := stored(t, conn, "household"); !bytes.Equal(got, replaced) {
		t.Error("the re-seal overwrote the secret the operator stored")
	}
	want := map[string]string{"personal": "household operator-secret", "work": "employer employer-secret"}
	if diff := cmp.Diff(want, secrets(loader.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the clients delta sync holds after reading one again (-want +got):\n%s", diff)
	}
	if !scan.Clients["household"] {
		t.Error("the scan reads the replaced secret, still on the old key, as done")
	}
	parts, err := seal.Split(stored(t, conn, "employer"))
	if err != nil {
		t.Fatal(err)
	}
	if parts.KeyID != seal.IDOf(newPublic.Bytes()) || scan.Clients["employer"] {
		t.Errorf("the client nobody replaced is on %s with its entry %v, want the current key and false", parts.KeyID, scan.Clients["employer"])
	}

	scan = loader.ResealClients(t.Context(), reseal.Writer(syncPool(t)))

	parts, err = seal.Split(stored(t, conn, "household"))
	if err != nil {
		t.Fatal(err)
	}
	if parts.KeyID != seal.IDOf(newPublic.Bytes()) || scan.Clients["household"] {
		t.Errorf("the next re-seal left the secret on %s with its entry %v, want the current key and false", parts.KeyID, scan.Clients["household"])
	}
	restarted := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler), []string{"gmail"})
	if err := restarted.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, secrets(restarted.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the clients after a restart (-want +got):\n%s", diff)
	}
}
