//go:build integration

package accountload_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// storedSecret reads a client's stored secret as the superuser.
func storedSecret(t *testing.T, conn *pgx.Conn, name string) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE name = $1", name).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// A write of a sealed client secret based on bytes since replaced, the re-seal of a client secret.
// Two clients of one provider are on an old key, and the operator replaces one of them after the
// loader loaded it. The re-seal does not overwrite it. The stored secret stays the operator's, the
// loader reads it again and holds it from then on for the account connecting through it, and only
// that account, and its scan entry follows the stored bytes. The other client, which nobody replaced,
// is re-sealed to the current key under its own name, and the account connecting through it keeps it
// (ADR-0089, ADR-0092, ADR-0106). The write is the compare-and-set on the bytes known the statement in
// db/oauthclients/secret makes, written here as the superuser.
func TestAReSealNeverPutsBackAReplacedSecret(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	old, current := generate(t), generate(t)
	ring := keyring(t, current, old)
	client(t, conn, "household", "gmail", "household-id", sealed(t, old, "first-secret", seal.ClientSecret("household")))
	client(t, conn, "employer", "gmail", "employer-id", sealed(t, old, "employer-secret", seal.ClientSecret("employer")))
	listedOnly(t, conn, "personal", "gmail")
	listedOnly(t, conn, "work", "gmail")
	through(t, conn, "personal", "household")
	through(t, conn, "work", "employer")
	var log logBuffer
	loader := loadWith(t, ring, &log, []string{"gmail"})
	replaced := sealed(t, old, "operator-secret", seal.ClientSecret("household"))
	must(t, conn, "UPDATE oauth_clients SET client_secret = $1 WHERE name = 'household'", replaced)
	write := func(ctx context.Context, name string, known, resealed []byte) (bool, error) {
		tag, err := conn.Exec(ctx, "UPDATE oauth_clients SET client_secret = $1 WHERE name = $2 AND client_secret = $3", resealed, name, known)
		return tag.RowsAffected() == 1, err
	}

	scan := loader.ResealClients(t.Context(), write)

	if got := storedSecret(t, conn, "household"); !bytes.Equal(got, replaced) {
		t.Error("the re-seal overwrote the secret the operator stored")
	}
	want := map[string]string{"personal": "household gmail household-id operator-secret", "work": "employer gmail employer-id employer-secret"}
	if diff := cmp.Diff(want, clients(loader.Snapshot()), compare.Options); diff != "" {
		t.Errorf("the clients the loader holds after reading one again (-want +got):\n%s", diff)
	}
	if !scan.Clients["household"] {
		t.Error("the scan reads the replaced secret, still on the old key, as done")
	}
	parts, err := seal.Split(storedSecret(t, conn, "employer"))
	if err != nil {
		t.Fatal(err)
	}
	if parts.KeyID != current.public.KeyID() || scan.Clients["employer"] {
		t.Errorf("the client nobody replaced is on %s with its entry %v, want the current key and false", parts.KeyID, scan.Clients["employer"])
	}
}
