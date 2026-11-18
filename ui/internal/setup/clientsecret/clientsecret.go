// Package clientsecret is the one part of the UI that opens a stored sealed value, and it opens only an
// OAuth client's secret (ADR-0081). Google refuses a desktop client's code exchange without the
// client's secret, PKCE notwithstanding, so the UI's code exchange needs the secret it stored sealed.
//
// It holds the opening of client secrets, open.ClientSecrets, over the keyring of private keys the
// deployables that call a provider open account credentials with, and never the keyring itself. Its
// one operation reads the named client's sealed secret and opens it bound to the client-secret
// purpose and that client's row. No exported function or method takes a sealing context or a sealed
// value, so no caller can ask it to open an account credential, and a credential's bytes copied into
// a client's row fail to open, since the purpose is part of what the value was sealed to (ADR-0088).
// The UI's import lists admit the opening half of the credential code here alone. That half links
// the keyring, so that this package loads only the opening of client secrets is held by review.
//
// The plaintext secret it returns lives for the exchange that asked for it. It is never logged, sent in
// a response, or put in the consent attempt's cookie.
package clientsecret

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
)

// Opener opens an OAuth client's secret from its row.
type Opener struct {
	secrets *open.ClientSecrets
	db      oauthclients.DBTX
}

// Load reads the public key and every private key from their mounted files, refusing a keyring whose
// public key matches none of its private keys, and reads the clients' rows through db.
func Load(publicKeyFile string, privateKeyFiles []string, db oauthclients.DBTX) (*Opener, error) {
	secrets, err := open.LoadClientSecrets(publicKeyFile, privateKeyFiles...)
	if err != nil {
		return nil, err
	}
	return &Opener{secrets: secrets, db: db}, nil
}

// ErrUnknownClient is a client no row holds.
var ErrUnknownClient = errors.New("clientsecret: no client holds this name")

// Secret reads the named client's sealed secret and opens it, bound to the client-secret purpose and
// the client's row.
func (o *Opener) Secret(ctx context.Context, client string) (string, error) {
	sealed, err := oauthclients.New(o.db).SealedClient(ctx, client)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownClient
	}
	if err != nil {
		return "", err
	}
	plain, err := o.secrets.Open(sealed, client)
	if err != nil {
		return "", fmt.Errorf("clientsecret: opening the secret of %s: %w", client, err)
	}
	return string(plain), nil
}
