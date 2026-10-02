//go:build banproof

package accountload

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
)

// This file runs statements beside the two of the txhelper analyser's exceptions that accountload
// uses on purpose, and banproof requires the wants below from go vet. The accounts listing and the read of oauth_clients may run on
// any handle only as one chained call. The listing's queries held in a variable, and the credential's
// statement chained the same way, are held to the transaction helper.
func besideTheExceptions(ctx context.Context, db DB) error {
	if _, err := accounts.New(db).Accounts(ctx); err != nil {
		return err
	}
	if _, err := oauthclients.New(db).OAuthClients(ctx); err != nil {
		return err
	}
	listing := accounts.New(db) // want vetcheck "calls accounts.New outside a function literal passed to tx.Run"
	if _, err := listing.Accounts(ctx); err != nil {
		return err
	}
	_, err := credential.New(db).Sealed(ctx, "personal") // want vetcheck "calls credential.New outside a function literal passed to tx.Run"
	return err
}
