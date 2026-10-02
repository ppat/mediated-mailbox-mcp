// Package accountload builds a deployable's account snapshot, the accounts it serves, each paired
// with the OAuth client it names where its provider has one, and the opened credentials,
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
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
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
	id       string
	provider string
	// clientName is the OAuth client the account's row names, empty when it names none.
	clientName string
	// client is that client, set only while its secret opened (ADR-0106).
	client    Client
	hasClient bool
	// withoutClient is set when the account's provider authenticates through an OAuth client and the
	// account has none whose secret opened, which leaves it not connected (ADR-0106).
	withoutClient bool
	credential    []byte
	adoption      uint64
}

// ID returns the account's identifier.
func (a Account) ID() string { return a.id }

// Provider returns the account's provider.
func (a Account) Provider() string { return a.provider }

// Client returns the OAuth client the account connects through, and whether it has one whose secret
// opened. It is the client the account's row names and never another of its provider, and an account
// whose provider authenticates without one has none (ADR-0106, ADR-0090).
func (a Account) Client() (Client, bool) { return a.client, a.hasClient }

// Connected reports whether the account has an opened credential and, where its provider
// authenticates through an OAuth client, the client it names. An account with no state row, no stored
// credential or a credential that did not open is not connected (ADR-0091), and neither is an account
// of such a provider that names no client or whose client's secret did not open (ADR-0106).
func (a Account) Connected() bool { return a.credential != nil && !a.withoutClient }

// Credential returns the account's opened credential, or nil when it is not connected. Its meaning is
// the provider adapter's (ADR-0016).
func (a Account) Credential() []byte {
	if !a.Connected() {
		return nil
	}
	return a.credential
}

// Adoption identifies the last value the loader adopted from the account's row that it did not write
// itself. A unit of work hands over, beside the credential it holds, the Adoption that came with the
// credential that one was built from, which is the account's Adoption when that credential was
// taken, from the snapshot or from a re-read the unit went on with. HandOver discards the hand-over
// once the account's Adoption has moved since (ADR-0089).
func (a Account) Adoption() uint64 { return a.adoption }

// Client is one of an installation's OAuth clients, which belongs to one provider (ADR-0106).
type Client struct {
	name     string
	provider string
	id       string
	secret   []byte
}

// Name returns the name the person running the installation gave the client.
func (c Client) Name() string { return c.name }

// Provider returns the provider the client serves.
func (c Client) Provider() string { return c.provider }

// ID returns the client's identifier.
func (c Client) ID() string { return c.id }

// Secret returns the client's opened secret.
func (c Client) Secret() []byte { return c.secret }

// Snapshot is one immutable account snapshot. A unit of work takes it once and keeps it (ADR-0090).
// Its zero value serves no account. It offers no lookup of a client by provider or by name, so an
// account reaches only the client its own row names, through Account.Client (ADR-0106).
type Snapshot struct {
	accounts []Account
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

// held is what the loader last knew of one stored sealed value and the plaintext it holds for it.
// baseline is the plaintext of known, the credential as it was last read or written. The plaintext
// is newer than the baseline while a write-back of it has not landed. adoption is the loader's count
// of outside values when it adopted known or the value known replaced by a write-back of its own.
type held struct {
	known     []byte
	baseline  []byte
	plaintext []byte
	adoption  uint64
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
	// adoptions counts the values adopted from a row that the loader did not write, across every
	// account, so an account dropped and connected again never reuses a count.
	adoptions uint64
	// clients holds, per client whose secret opened, keyed on its name, the sealed bytes last read or
	// written.
	clients map[string][]byte
	scan    Scan
	active  atomic.Pointer[Snapshot]
	// throughClient holds the providers the deployable names as authenticating through an OAuth client.
	throughClient map[string]bool
}

// New returns a Loader reading from db and opening with keys. Its snapshot serves no account until a
// load succeeds. throughClient names the providers that authenticate through an OAuth client, which
// the deployable knows from its adapters and the library does not. An account of one of them that
// names no client, or whose client's secret did not open, is not connected (ADR-0106).
func New(db DB, keys *open.Keyring, log *slog.Logger, throughClient []string) *Loader {
	l := &Loader{
		db: db, keys: keys, log: log, accounts: map[string]held{}, clients: map[string][]byte{}, scan: newScan(),
		throughClient: map[string]bool{},
	}
	for _, p := range throughClient {
		l.throughClient[p] = true
	}
	l.active.Store(&Snapshot{})
	return l
}

// Snapshot returns the active snapshot.
func (l *Loader) Snapshot() *Snapshot { return l.active.Load() }

// row is one listed account with the client it names and what its state row holds.
type row struct {
	id         string
	provider   string
	clientName string
	stored     []byte
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
		rows = append(rows, row{id: a.AccountID, provider: a.AccountProvider, clientName: a.OauthClient.String, stored: stored})
	}
	return rows, clients, nil
}

// stored reads the account's sealed credential in its own transaction. An account with no state row
// or no credential reads as nil.
func (l *Loader) stored(ctx context.Context, account string) ([]byte, error) {
	var stored []byte
	err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
		read, err := credential.New(t).Sealed(ctx, account)
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
// credential that does not open is logged and leaves its account not connected, and a client secret
// that does not open is logged and leaves every account naming the client without one.
func (l *Loader) Load(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, clients, err := l.read(ctx)
	if err != nil {
		l.log.Error("the account snapshot was not reloaded, so the previous one stays", slog.Any("error", err))
		return fmt.Errorf("%w: %w", ErrUntrustedRead, err)
	}
	next := &Snapshot{}
	known := map[string]held{}
	opened := map[string]Client{}
	sealedClients := map[string][]byte{}
	scan := newScan()
	for _, c := range clients {
		client, ok := l.openClient(c)
		scan.Clients[c.ClientName] = !ok || l.onOldKey(c.SealedClientSecret)
		if !ok {
			continue
		}
		opened[c.ClientName] = client
		sealedClients[c.ClientName] = c.SealedClientSecret
	}
	for _, r := range rows {
		a := l.paired(Account{id: r.id, provider: r.provider}, r.clientName, opened)
		h, ok := l.adopt(r.id, r.stored)
		if ok {
			a.credential, a.adoption = h.plaintext, h.adoption
			known[r.id] = h
		}
		scan.Accounts[r.id] = r.stored != nil && (!ok || l.onOldKey(r.stored))
		next.accounts = append(next.accounts, a)
	}
	l.accounts = known
	l.clients = sealedClients
	l.scan = scan
	l.active.Store(next)
	return nil
}

// withoutClient reports whether the account's provider authenticates through an OAuth client while
// the account has none whose secret opened, and logs each such account.
func (l *Loader) withoutClient(a Account) bool {
	if !l.throughClient[a.provider] || a.hasClient {
		return false
	}
	l.log.Warn("an account's provider authenticates through an OAuth client and the account has none that opened, so it is not connected",
		slog.String("account", a.id), slog.String("provider", a.provider), slog.String("client", a.clientName))
	return true
}

// openClient opens a stored client's secret, bound to the client's row by its name. A secret that
// does not open is logged, and the client is not served.
func (l *Loader) openClient(c oauthclients.OAuthClientsRow) (Client, bool) {
	secret, err := l.keys.Open(c.SealedClientSecret, seal.ClientSecret(c.ClientName))
	if err != nil {
		l.log.Error("an OAuth client's secret did not open, so no account connects through it", slog.String("client", c.ClientName), slog.Any("error", err))
		return Client{}, false
	}
	return Client{name: c.ClientName, provider: c.Provider, id: c.ClientID, secret: secret}, true
}

// paired returns the account paired with the client its row names, from the clients whose secrets
// opened, keyed on their names, and left not connected when its provider authenticates through a
// client and it has none. Every place the loader pairs an account with its client goes through it
// (ADR-0106).
func (l *Loader) paired(a Account, name string, opened map[string]Client) Account {
	a.clientName = name
	a.client, a.hasClient = pair(name, opened)
	a.withoutClient = l.withoutClient(a)
	return a
}

// pair returns the client an account's row names, from the clients whose secrets opened, keyed on
// their names. An account that names none has none.
func pair(name string, opened map[string]Client) (Client, bool) {
	if name == "" {
		return Client{}, false
	}
	c, ok := opened[name]
	return c, ok
}

// onOldKey reports whether a stored value names a key other than the current one.
func (l *Loader) onOldKey(stored []byte) bool {
	parts, err := seal.Split(stored)
	return err != nil || parts.KeyID != l.keys.Current()
}

// adopt returns what the loader holds for an account whose state row holds stored. A value it
// already knew keeps the plaintext it holds, and any other is opened and counted as an outside
// value.
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
	l.adoptions++
	return held{known: stored, baseline: plaintext, plaintext: plaintext, adoption: l.adoptions}, true
}
