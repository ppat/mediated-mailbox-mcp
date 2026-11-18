package open_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// The opening of client secrets opens a value sealed as the named client's secret, and refuses one
// sealed as an account's credential under the same name and one sealed as another client's secret,
// so the UI's one opening part cannot be made to open a credential (ADR-0081, ADR-0088,
// VERIFICATIONS, the UI's open of a credential row). It exposes no field to reach the keyring
// through.
func TestClientSecretsOpenAClientsSecretAndNothingElse(t *testing.T) {
	seed, public := pair(t)
	dir := t.TempDir()
	publicFile, privateFile := filepath.Join(dir, "public.key"), filepath.Join(dir, "private.key")
	for path, b := range map[string][]byte{publicFile: public, privateFile: seed} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	secrets, err := open.LoadClientSecrets(publicFile, privateFile)
	if err != nil {
		t.Fatal(err)
	}
	key := publicKey(t, public)
	sealed := func(c seal.Context) []byte {
		t.Helper()
		b, err := key.Seal([]byte("the secret"), c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	plain, err := secrets.Open(sealed(seal.ClientSecret("household")), "household")
	if err != nil || string(plain) != "the secret" {
		t.Fatalf("the client's own secret opened to %q, %v", plain, err)
	}
	for name, c := range map[string]seal.Context{
		"an account's credential": seal.AccountCredential("household"),
		"another client's secret": seal.ClientSecret("spare-client"),
	} {
		if plain, err := secrets.Open(sealed(c), "household"); !errors.Is(err, open.ErrRefused) {
			t.Errorf("a value sealed as %s opened as the client's secret to %q, %v", name, plain, err)
		}
	}
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/credential/open", "ClientSecrets")
}
