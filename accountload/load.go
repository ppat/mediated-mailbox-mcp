// Package accountload builds a deployable's account snapshot, the accounts it serves, the
// installation's OAuth client for each of their providers that has one and the opened credentials,
// writes a rotated credential back by compare-and-set, and carries delta sync's re-seal and the scan
// of what is still sealed to an old key (ADR-0089, ADR-0090, ADR-0092). Its case is argued in its
// README.
package accountload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// DB is what the loader reads and writes through, a pool in a deployable. The listing and the OAuth
// clients belong to no account and are read outside an account's transaction. Each account's state is
// read and written inside its own (ADR-0047, ADR-0091).
type DB interface {
	tx.Beginner
	accounts.DBTX
}

// ErrUntrustedRead wraps the failure of a reload whose read failed. The previous snapshot stays.
var ErrUntrustedRead = errors.New("the account snapshot's read failed, so the previous snapshot stays")

// Account is one account of a snapshot. A zero Account is not connected.
type Account struct {
	id         string
	provider   string
	credential []byte
}

// ID returns the account's identifier.
func (a Account) ID() string { return a.id }

// Provider returns the account's provider.
func (a Account) Provider() string { return a.provider }

// Connected reports whether the account has an opened credential. An account with no state row, no
// stored credential or a credential that did not open is not connected (ADR-0091).
func (a Account) Connected() bool { return a.credential != nil }

// Credential returns the account's opened credential, or nil when it is not connected. Its meaning is
// the provider adapter's (ADR-0016).
func (a Account) Credential() []byte { return a.credential }

// Client is an installation's OAuth client for one provider (ADR-0083).
type Client struct {
	provider string
	id       string
	secret   []byte
}

// Provider returns the provider the client serves.
func (c Client) Provider() string { return c.provider }

// ID returns the client's identifier.
func (c Client) ID() string { return c.id }

// Secret returns the client's opened secret.
func (c Client) Secret() []byte { return c.secret }

// Snapshot is one immutable account snapshot. A unit of work takes it once and keeps it (ADR-0090).
// Its zero value serves no account.
type Snapshot struct {
	accounts []Account
	clients  map[string]Client
}

// Accounts returns the snapshot's accounts in identifier order.
func (s *Snapshot) Accounts() []Account { return slices.Clone(s.accounts) }

// Account returns the account with the identifier, and whether the snapshot serves it.
func (s *Snapshot) Account(id string) (Account, bool) {
	i, found := slices.BinarySearchFunc(s.accounts, id, func(a Account, id string) int {
		switch {
		case a.id < id:
			return -1
		case a.id > id:
			return 1
		default:
			return 0
		}
	})
	if !found {
		return Account{}, false
	}
	return s.accounts[i], true
}

// Client returns the OAuth client of the provider, and whether the provider has one that opened. A
// provider that authenticates without an OAuth client has none, and nothing looks for one.
func (s *Snapshot) Client(provider string) (Client, bool) {
	c, ok := s.clients[provider]
	return c, ok
}

// held is what the loader last knew of one stored sealed value and the plaintext it holds for it.
// baseline is the plaintext of known, the credential as it was last read or written. The plaintext
// is newer than the baseline while a write-back of it has not landed.
type held struct {
	known     []byte
	baseline  []byte
	plaintext []byte
}

// Loader holds the active snapshot and what it last knew of each stored value. It is safe for use by
// several goroutines.
type Loader struct {
	db   DB
	keys *open.Keyring
	log  *slog.Logger

	// mu serialises reloads, write-backs and re-seals, so what the loader knows of a stored value moves
	// with one of them at a time.
	mu       sync.Mutex
	accounts map[string]held
	scan     Scan
	active   atomic.Pointer[Snapshot]
}

// New returns a Loader reading from db and opening with keys. Its snapshot serves no account until a
// load succeeds.
func New(db DB, keys *open.Keyring, log *slog.Logger) *Loader {
	l := &Loader{db: db, keys: keys, log: log, accounts: map[string]held{}, scan: newScan()}
	l.active.Store(&Snapshot{})
	return l
}

// Snapshot returns the active snapshot.
func (l *Loader) Snapshot() *Snapshot { return l.active.Load() }

// row is one listed account with what its state row holds.
type row struct {
	id       string
	provider string
	stored   []byte
}

// read lists the accounts and the OAuth clients and reads each account's stored credential. It
// returns nothing with any error.
func (l *Loader) read(ctx context.Context) ([]row, []oauthclients.OAuthClientsRow, error) {
	var listed []accounts.AccountsRow
	var clients []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, l.db, func(t pgx.Tx) error {
		var err error
		if listed, err = accounts.New(t).Accounts(ctx); err != nil {
			return fmt.Errorf("listing the accounts: %w", err)
		}
		if clients, err = oauthclients.New(t).OAuthClients(ctx); err != nil {
			return fmt.Errorf("reading the OAuth clients: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	rows := make([]row, 0, len(listed))
	for _, a := range listed {
		stored, err := l.stored(ctx, a.AccountID)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, row{id: a.AccountID, provider: a.AccountProvider, stored: stored})
	}
	return rows, clients, nil
}

// stored reads the account's sealed credential in its own transaction. An account with no state row
// or no credential reads as nil.
func (l *Loader) stored(ctx context.Context, account string) ([]byte, error) {
	var stored []byte
	err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
		read, err := accountstate.New(t).Sealed(ctx, account)
		if err != nil {
			return err
		}
		if len(read) == 1 {
			stored = read[0]
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("account %s: reading its credential: %w", account, err)
	}
	return stored, nil
}

// Load reads the accounts, their credentials and the OAuth clients, opens them and makes the result
// the active snapshot. A read that fails keeps the previous snapshot, is logged and returns an error
// wrapping ErrUntrustedRead. A read that lists no account serves none (ADR-0090).
//
// A credential whose stored bytes are the ones the loader last knew keeps the plaintext it holds,
// which is newer when a rotated credential's write-back has not landed. Stored bytes the loader did
// not know are opened and replace what it held, since someone else replaced the value (ADR-0089). A
// credential or client secret that does not open is logged and leaves its account not connected or
// its provider without a client.
func (l *Loader) Load(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, clients, err := l.read(ctx)
	if err != nil {
		l.log.Error("the account snapshot was not reloaded, so the previous one stays", slog.Any("error", err))
		return fmt.Errorf("%w: %w", ErrUntrustedRead, err)
	}
	next := &Snapshot{clients: map[string]Client{}}
	known := map[string]held{}
	scan := newScan()
	for _, r := range rows {
		a := Account{id: r.id, provider: r.provider}
		h, ok := l.adopt(r.id, r.stored)
		if ok {
			a.credential = h.plaintext
			known[r.id] = h
		}
		scan.Accounts[r.id] = r.stored != nil && (!ok || l.onOldKey(r.stored))
		next.accounts = append(next.accounts, a)
	}
	for _, c := range clients {
		secret, err := l.keys.Open(c.SealedClientSecret, seal.ClientSecret(c.Provider))
		scan.Clients[c.Provider] = err != nil || l.onOldKey(c.SealedClientSecret)
		if err != nil {
			l.log.Error("an OAuth client's secret did not open, so its provider has no client", slog.String("provider", c.Provider), slog.Any("error", err))
			continue
		}
		next.clients[c.Provider] = Client{provider: c.Provider, id: c.ClientID, secret: secret}
	}
	l.accounts = known
	l.scan = scan
	l.active.Store(next)
	return nil
}

// onOldKey reports whether a stored value names a key other than the current one.
func (l *Loader) onOldKey(stored []byte) bool {
	parts, err := seal.Split(stored)
	return err != nil || parts.KeyID != l.keys.Current()
}

// adopt returns what the loader holds for an account whose state row holds stored. A value it
// already knew keeps the plaintext it holds, and any other is opened.
func (l *Loader) adopt(account string, stored []byte) (held, bool) {
	if stored == nil {
		return held{}, false
	}
	if h, ok := l.accounts[account]; ok && slices.Equal(h.known, stored) {
		return h, true
	}
	plaintext, err := l.keys.Open(stored, seal.AccountCredential(account))
	if err != nil {
		l.log.Error("an account's credential did not open, so it is not connected", slog.String("account", account), slog.Any("error", err))
		return held{}, false
	}
	return held{known: stored, baseline: plaintext, plaintext: plaintext}, true
}
