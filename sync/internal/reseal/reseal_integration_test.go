//go:build integration

package reseal_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal"
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

func sealed(t *testing.T, to seal.PublicKey, plaintext string) []byte {
	t.Helper()
	b, err := to.Seal([]byte(plaintext), seal.ClientSecret("gmail"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func stored(t *testing.T, conn *pgx.Conn) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE provider = 'gmail'").Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// D4's part of VERIFICATIONS' row for a write of a sealed value based on bytes since replaced, delta
// sync's re-seal of a client secret. A secret the operator replaces after delta sync loaded it is not
// overwritten by the re-seal. The stored secret stays the operator's, delta sync reads it again and
// holds it from then on, and its scan entry follows the stored bytes. A secret nobody replaced is
// re-sealed to the current key and its entry reads false (ADR-0089, ADR-0092).
func TestAReSealNeverPutsBackAReplacedSecret(t *testing.T) {
	conn := superuser(t)
	oldPrivate, oldPublic := keyPair(t)
	newPrivate, newPublic := keyPair(t)
	ring, err := open.NewKeyring(newPublic, oldPrivate, newPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(t.Context(), "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ('gmail', 'client-id', $1)", sealed(t, oldPublic, "first-secret")); err != nil {
		t.Fatal(err)
	}
	loader := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler))
	if err := loader.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	replaced := sealed(t, oldPublic, "operator-secret")
	if _, err := conn.Exec(t.Context(), "UPDATE oauth_clients SET client_secret = $1", replaced); err != nil {
		t.Fatal(err)
	}

	scan := loader.ResealClients(t.Context(), reseal.Writer(syncPool(t)))

	if got := stored(t, conn); !bytes.Equal(got, replaced) {
		t.Error("the re-seal overwrote the secret the operator stored")
	}
	if c, ok := loader.Snapshot().Client("gmail"); !ok || string(c.Secret()) != "operator-secret" {
		t.Errorf("delta sync holds the client %v, want the operator's secret read again", ok)
	}
	if !scan.Clients["gmail"] {
		t.Error("the scan reads the replaced secret, still on the old key, as done")
	}

	scan = loader.ResealClients(t.Context(), reseal.Writer(syncPool(t)))

	parts, err := seal.Split(stored(t, conn))
	if err != nil {
		t.Fatal(err)
	}
	if parts.KeyID != seal.IDOf(newPublic.Bytes()) || scan.Clients["gmail"] {
		t.Errorf("the next re-seal left the secret on %s with its entry %v, want the current key and false", parts.KeyID, scan.Clients["gmail"])
	}
	restarted := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler))
	if err := restarted.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c, ok := restarted.Snapshot().Client("gmail"); !ok || string(c.Secret()) != "operator-secret" {
		t.Errorf("after a restart the client opens %v, want the operator's secret", ok)
	}
}
