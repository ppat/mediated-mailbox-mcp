//go:build integration

package clientsecret_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// connect opens a connection as the role, or as the superuser for an empty role.
func connect(t *testing.T, role string) *pgx.Conn {
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
	if role != "" {
		if _, err := conn.Exec(t.Context(), "SET ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	return conn
}

// The opener opens a client's secret sealed to its own row, as the UI's role reads it. A value sealed
// as an account's credential and copied into a client's row, and a secret sealed to another client's
// row, are each refused, so the one part of the UI that opens a stored value cannot be made to open a
// credential (ADR-0081, ADR-0088, VERIFICATIONS, the UI's open of a credential row).
func TestTheOpenerOpensAClientsSecretAndNothingElse(t *testing.T) {
	private, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := private.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	public, err := seal.ParsePublicKey(private.PublicKey().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	publicFile, privateFile := filepath.Join(dir, "public.key"), filepath.Join(dir, "private.key")
	for path, b := range map[string][]byte{publicFile: public.Bytes(), privateFile: seed} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sealed := func(plaintext string, c seal.Context) []byte {
		t.Helper()
		b, err := public.Seal([]byte(plaintext), c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	admin := connect(t, "")
	for _, row := range []struct {
		name   string
		secret []byte
	}{
		{"household", sealed("the household secret", seal.ClientSecret("household"))},
		{"credential-copy", sealed("1//a refresh token", seal.AccountCredential("credential-copy"))},
		{"moved-secret", sealed("the household secret", seal.ClientSecret("household"))},
	} {
		if _, err := admin.Exec(t.Context(), "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, 'gmail', $1, $2)", row.name, row.secret); err != nil {
			t.Fatal(err)
		}
	}
	opener, err := clientsecret.Load(publicFile, []string{privateFile}, connect(t, "mediated_mailbox_ui"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := opener.Secret(t.Context(), "household"); err != nil || got != "the household secret" {
		t.Errorf("the client's own secret opened to %q, %v", got, err)
	}
	for _, name := range []string{"credential-copy", "moved-secret"} {
		if got, err := opener.Secret(t.Context(), name); err == nil {
			t.Errorf("%s opened to %q, want it refused", name, got)
		}
	}
	if _, err := opener.Secret(t.Context(), "absent"); !errors.Is(err, clientsecret.ErrUnknownClient) {
		t.Errorf("an absent client: %v, want %v", err, clientsecret.ErrUnknownClient)
	}
}
