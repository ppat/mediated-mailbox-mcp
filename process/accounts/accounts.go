// Package accounts is the reading of the list of accounts a process's Session holds, each with its
// provider and the name of the OAuth client it connects through, as one read of the accounts table
// gives them (ADR-0091, ADR-0106). It holds no credential. A read that fails is a failure, never a
// list with no account, so the Session keeps the list in force, and a read that succeeds with no
// account is the empty list (ADR-0090). Its case is argued in process/README.md.
package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
)

// Account is one listed account. A zero Client is an account that names no OAuth client, as an
// account of a provider without one does.
type Account struct {
	// ID is the account's identifier.
	ID string
	// Provider is the provider the account names, as the provider library names its adapters.
	Provider string
	// Client is the name of the OAuth client the account's row names, empty for none (ADR-0106).
	Client string
}

// ErrUntrustedRead wraps the failure of a read of the list, so the list in force stays.
var ErrUntrustedRead = errors.New("the read of the accounts failed, so the list in force stays")

// Read returns every listed account in identifier order, through db, outside any account's
// transaction, since the list belongs to no account (ADR-0091). A read that fails returns no account
// and an error wrapping ErrUntrustedRead.
func Read(ctx context.Context, db accounts.DBTX) ([]Account, error) {
	rows, err := accounts.New(db).Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: listing the accounts: %w", ErrUntrustedRead, err)
	}
	listed := make([]Account, 0, len(rows))
	for _, r := range rows {
		listed = append(listed, Account{ID: r.AccountID, Provider: r.AccountProvider, Client: r.OauthClient.String})
	}
	return listed, nil
}
