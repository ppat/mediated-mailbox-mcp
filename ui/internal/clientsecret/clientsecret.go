// Package clientsecret is the one part of the UI that opens a stored sealed value, and it opens only an
// OAuth client's secret (ADR-0081). Google refuses a desktop client's code exchange without the
// client's secret, PKCE notwithstanding, so the UI's code exchange needs the secret it stored sealed.
//
// It holds the keyring of private keys, the same keys the deployables that call a provider open
// account credentials with, and its one operation opens the named client's secret bound to the
// client-secret purpose and that client's row. No exported function or method takes a sealing context
// or a sealed value, so no caller can ask it to open an account credential, and a credential's bytes
// copied into a client's row fail to open, since the purpose is part of what the value was sealed to
// (ADR-0088). The UI's import lists admit the opening half of the credential code here alone.
//
// The plaintext secret it returns lives for the exchange that asked for it. It is never logged, sent in
// a response, or put in the consent attempt's cookie.
package clientsecret

import (
	"context"
	"errors"
	"fmt"

	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
)

// Opener opens an OAuth client's secret from its row.
type Opener struct {
	ring *open.Keyring
	db   oauthclients.DBTX
}

// Load reads the public key and every private key from their mounted files, refusing a keyring whose
// public key matches none of its private keys, and reads the clients' rows through db.
func Load(publicKeyFile string, privateKeyFiles []string, db oauthclients.DBTX) (*Opener, error) {
	ring, err := open.Load(publicKeyFile, privateKeyFiles...)
	if err != nil {
		return nil, err
	}
	return &Opener{ring: ring, db: db}, nil
}

// ErrUnknownClient is a client no row holds.
var ErrUnknownClient = errors.New("clientsecret: no client holds this name")

// Secret opens the named client's secret, bound to the client-secret purpose and the client's row.
func (o *Opener) Secret(ctx context.Context, client string) (string, error) {
	rows, err := oauthclients.New(o.db).OAuthClients(ctx)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.ClientName != client {
			continue
		}
		plain, err := o.ring.Open(r.SealedClientSecret, seal.ClientSecret(client))
		if err != nil {
			return "", fmt.Errorf("clientsecret: opening the secret of %s: %w", client, err)
		}
		return string(plain), nil
	}
	return "", ErrUnknownClient
}
