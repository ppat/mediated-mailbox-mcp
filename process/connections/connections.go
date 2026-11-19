// Package connections is each account's connection to its provider as a process holds it, one record
// per account, attached to the process's Session as its third member, with the unit of work, the
// compare-and-set write-back of a rotated credential and delta sync's re-seal (ADR-0089, ADR-0090,
// ADR-0092, ADR-0097, ADR-0106, ADR-0142). It opens each stored credential and OAuth client secret
// with the keys it loads, makes each account's token through the provider library's list of
// adapters, and keeps the token across reloads while the stored credential is the one it pairs with.
// None of a record is exported. Its case is argued in process/README.md.
package connections

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/accounts"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
)

// ErrUntrustedRead wraps the failure of a load whose read of the credentials or the OAuth clients
// failed, so the connections in force stay.
var ErrUntrustedRead = errors.New("the read of the connections failed, so the connections in force stay")

// ErrNotConnected is an account no unit of work may begin on, one the list in force does not hold,
// one with no opened credential, one whose provider authenticates through an OAuth client it has
// none of, or one whose provider no adapter serves.
var ErrNotConnected = errors.New("the account has no connection a unit of work may begin on")

// held is the slot the process keeps its connections in, and shipped the slot it keeps the list of
// adapters they make tokens and mailboxes from, which the one option for a test fills.
var (
	held    = process.NewOnePerProcess[*Connections]("connections")
	shipped = process.NewOnePerProcess[provider.Adapters]("adapters")
)

// WithAdapters returns the option a test or a harness runs a process with so its connections make
// every token and mailbox from list, such as the adapters over the provider fake, in place of the
// shipped adapters. It is the connections' one option for a test (ADR-0142). process.Run takes no
// option, so no program can select it.
func WithAdapters(list provider.Adapters) process.Option { return shipped.Substitute(list) }

// Connections are the process's connections, one record per account the Session lists. They are safe
// for concurrent use.
type Connections struct {
	db       *pgxpool.Pool
	keys     *open.Keyring
	adapters provider.Adapters
	session  *sessions.Session
	log      *slog.Logger

	// mu serialises loads, re-reads, write-backs and re-seals across their database statements, so
	// what the connections know of a stored value moves with one of them at a time and no read takes
	// the process's own write for someone else's (ADR-0089). Beginning a unit
	// of work never takes it, and a unit's calls take it only for the re-read a refused call makes.
	// Where both are held it is taken before a record's lock.
	mu sync.Mutex
	// clients holds, per client whose secret opened, keyed on its name, the sealed bytes last read or
	// written, and opened the client itself.
	clients map[string][]byte
	opened  map[string]coremail.OAuthClient
	scan    Scan
	// adoptions counts the values adopted from an account's row, across every account, so a record's
	// stamp never comes back to a value it held.
	adoptions atomic.Uint64
	// records is replaced whole at each load, so a Begin reads a whole map. A record listed in two
	// loads is the same record.
	records atomic.Pointer[map[string]*record]
}

// record is one account's connection. Its fields are guarded by its lock, so a unit of work's end
// reads its token's credential and its adoption stamp together.
type record struct {
	mu       sync.Mutex
	provider string
	// clientName is the OAuth client the account's row names, empty for none, and client that
	// client, set while hasClient, which holds only while its secret opened (ADR-0106).
	clientName string
	client     coremail.OAuthClient
	hasClient  bool
	// stored is the account's stored credential as the connections last read or wrote it, the bytes
	// a write-back compares against. baseline is its plaintext.
	stored   []byte
	baseline []byte
	// token is what the account's units of work call with, made from client and the credential, and
	// kept while stored is the value it pairs with. It is nil while the account is not connected.
	token     *provider.Token
	connected bool
	// stamp is the record's adoption stamp, which moves each time the connections adopt a value from
	// the account's row that they did not write themselves, and which their own write-back leaves as
	// it is (ADR-0089).
	stamp uint64
	// target is the account's lowered rate target, read at each load while it is connected. A unit of
	// work begins only once one was read without an error.
	targetRead bool
	target     float64
	lowered    bool
	targetErr  error
}

// Of returns the process's connections, built the first time any part asks, from the process, with
// the process's own pool and logger, and attached to s as its third member, and the same ones on
// every later call. The build loads the public key and every private key c's credential part names,
// so a public key matching none of them stops the start (ADR-0081, ADR-0088, ADR-0092). Each token
// and mailbox is made from the shipped adapters, or from those WithAdapters handed the process.
func Of(ctx context.Context, p process.Capabilities, c config.HasConnections, s *sessions.Session) (*Connections, error) {
	return held.Get(ctx, p, func(ctx context.Context, shared process.Capabilities) (*Connections, func(), error) {
		list, err := shipped.Get(ctx, shared, func(context.Context, process.Capabilities) (provider.Adapters, func(), error) {
			return provider.Shipped(http.DefaultClient), nil, nil
		})
		if err != nil {
			return nil, nil, err
		}
		pool, err := database.Get(ctx, shared, c)
		if err != nil {
			return nil, nil, err
		}
		k := c.KeysPart()
		keys, err := open.Load(k.PublicKeyFile, k.PrivateKeyFiles...)
		if err != nil {
			return nil, nil, fmt.Errorf("loading the keyring: %w", err)
		}
		conns := &Connections{
			db: pool, keys: keys, adapters: list, session: s, log: shared.Logger(),
			clients: map[string][]byte{}, opened: map[string]coremail.OAuthClient{}, scan: newScan(),
		}
		conns.records.Store(&map[string]*record{})
		if err := s.Attach(conns); err != nil {
			return nil, nil, err
		}
		return conns, nil, nil
	})
}

// Caller is one part's caller of a provider, a job kind's or the API server's body operation's, with
// the pool of that part, which its leases run on and the units of work begun through it write back
// and record on (ADR-0097, ADR-0117, ADR-0119).
type Caller struct {
	calls *provider.Caller
	pool  tx.Beginner
}

// Caller returns a caller of the part p was handed to, over the connections' adapters, its series
// registered on p's registerer, and pool the part's own pool, the one database.Get gives the part, so
// a job kind's or an operation's provider calls are counted on its own series and its leases,
// write-backs and recorded attempts run on its own pool. A job kind hands its own.
func (c *Connections) Caller(p process.Capabilities, pool tx.Beginner, o provider.Options) (*Caller, error) {
	calls, err := provider.NewCaller(c.adapters, p.Metrics(), pool, o)
	if err != nil {
		return nil, err
	}
	return &Caller{calls: calls, pool: pool}, nil
}

// Connected reports whether a unit of work may begin on the account, which the Session reads to say
// which accounts are workable. A connected account whose lowered target could not be read counts,
// and each unit of work on it is refused.
func (c *Connections) Connected(account string) bool {
	rec, ok := (*c.records.Load())[account]
	if !ok {
		return false
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.connected
}

// row is one listed account with what its state row holds.
type row struct {
	account accounts.Account
	stored  []byte
}

// Load loads the connections of the accounts listed, as the Session's reload asks. It reads the OAuth
// clients and each account's stored credential, and a read that fails keeps the connections in force,
// is logged and returns an error wrapping ErrUntrustedRead. An account no longer listed loses its
// record (ADR-0090).
//
// A credential whose stored bytes are the ones the record pairs its token with keeps the token, which
// holds a rotated credential while its write-back has not landed. Stored bytes the connections did not
// know are opened, and a token made from them replaces the record's, since someone else replaced the
// value (ADR-0089). A credential that does not open is logged and leaves its account not connected, and
// a client secret that does not open is logged and leaves every account naming the client without one.
// Each connected account's lowered rate target is read again, and one that cannot be read is logged and
// kept, so no unit of work begins on that account until a load reads it (ADR-0024).
func (c *Connections) Load(ctx context.Context, listed []accounts.Account) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows, clients, err := c.read(ctx, listed)
	if err != nil {
		c.log.Error("the connections were not reloaded, so the previous ones stay", slog.Any("error", err))
		return fmt.Errorf("%w: %w", ErrUntrustedRead, err)
	}
	opened := map[string]coremail.OAuthClient{}
	sealedClients := map[string][]byte{}
	scan := newScan()
	for _, cl := range clients {
		client, ok := c.openClient(cl)
		scan.Clients[cl.ClientName] = !ok || c.onOldKey(cl.SealedClientSecret)
		if !ok {
			continue
		}
		opened[cl.ClientName] = client
		sealedClients[cl.ClientName] = cl.SealedClientSecret
	}
	previous := *c.records.Load()
	next := make(map[string]*record, len(rows))
	for _, r := range rows {
		rec, ok := previous[r.account.ID]
		if !ok {
			rec = &record{}
		}
		adopted := c.adopt(rec, r.account, opened, r.stored)
		scan.Accounts[r.account.ID] = r.stored != nil && (!adopted || c.onOldKey(r.stored))
		next[r.account.ID] = rec
	}
	for _, r := range rows {
		c.readTarget(ctx, r.account.ID, next[r.account.ID])
	}
	c.clients, c.opened, c.scan = sealedClients, opened, scan
	c.records.Store(&next)
	return nil
}

// read reads the OAuth clients and each listed account's stored credential. It returns nothing with
// any error.
func (c *Connections) read(ctx context.Context, listed []accounts.Account) ([]row, []oauthclients.OAuthClientsRow, error) {
	clients, err := c.readClients(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows := make([]row, 0, len(listed))
	for _, a := range listed {
		stored, err := c.stored(ctx, a.ID)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, row{account: a, stored: stored})
	}
	return rows, clients, nil
}

// readClients reads the stored OAuth clients, outside any account's transaction, since the table
// belongs to no account (ADR-0091).
func (c *Connections) readClients(ctx context.Context) ([]oauthclients.OAuthClientsRow, error) {
	var clients []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, c.db, func(t pgx.Tx) error {
		var err error
		clients, err = oauthclients.New(t).OAuthClients(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("reading the OAuth clients: %w", err)
	}
	return clients, nil
}

// stored reads the account's sealed credential in its own transaction. An account with no state row
// or no credential reads as nil.
func (c *Connections) stored(ctx context.Context, account string) ([]byte, error) {
	return storedOn(ctx, c.db, account)
}

// storedOn reads the account's sealed credential through db, as stored does.
func storedOn(ctx context.Context, db tx.Beginner, account string) ([]byte, error) {
	var stored []byte
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		read, err := credential.New(t).Sealed(ctx)
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

// adopt sets rec from the listed account a, paired with the client its row names from the clients
// whose secrets opened, keyed on their names, and stored, its state row's sealed credential, and
// reports whether the credential is one the connections hold opened. The record keeps its token
// while the client is unchanged and the stored value is the one the record knew or opens to the
// credential the token holds now, so a refresh written back, or the same credential stored under new
// bytes, keeps the token. Any other value is opened, and a token made from it replaces the record's. The caller holds the connections' lock.
func (c *Connections) adopt(rec *record, a accounts.Account, opened map[string]coremail.OAuthClient, stored []byte) bool {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	client, hasClient := pair(a.Client, opened)
	clientChanged := rec.hasClient != hasClient || rec.client != client
	rec.provider, rec.clientName, rec.client, rec.hasClient = a.Provider, a.Client, client, hasClient
	if stored == nil {
		rec.disconnect()
		return false
	}
	plaintext, same := rec.current(), rec.stored != nil && bytes.Equal(rec.stored, stored)
	if !same {
		var err error
		if plaintext, err = c.keys.Open(stored, seal.AccountCredential(a.ID)); err != nil {
			c.log.Error("an account's credential did not open, so it is not connected", slog.String("account", a.ID), slog.Any("error", err))
			rec.disconnect()
			return false
		}
	}
	c.adopted(rec, same)
	if !c.servable(a.ID, rec) {
		rec.stored, rec.baseline, rec.token, rec.connected = stored, baselineOf(rec, same, plaintext), nil, false
		return true
	}
	if !clientChanged && rec.token != nil && (same || bytes.Equal(plaintext, rec.token.Credential())) {
		rec.stored, rec.baseline, rec.connected = stored, baselineOf(rec, same, plaintext), true
		return true
	}
	token, err := c.adapters.NewToken(rec.provider, rec.client, plaintext)
	if err != nil {
		c.log.Error("an account's token could not be made, so it is not connected", slog.String("account", a.ID), slog.Any("error", err))
		rec.stored, rec.baseline, rec.token, rec.connected = stored, baselineOf(rec, same, plaintext), nil, false
		return true
	}
	rec.stored, rec.baseline, rec.token, rec.connected = stored, baselineOf(rec, same, plaintext), token, true
	return true
}

// adopted gives the record a new stamp for a value adopted from the account's row, when it is not
// the value the record already held. The caller holds the record's lock.
func (c *Connections) adopted(rec *record, same bool) {
	if !same {
		rec.stamp = c.adoptions.Add(1)
	}
}

// baselineOf returns the plaintext of the stored bytes, which a value the record already knew keeps
// and a value just opened sets.
func baselineOf(rec *record, same bool, opened []byte) []byte {
	if same {
		return rec.baseline
	}
	return opened
}

// current returns the credential the record's token holds now, a rotation whose write-back has not
// landed included, or the plaintext of the stored bytes while it has no token.
func (rec *record) current() []byte {
	if rec.token != nil {
		return rec.token.Credential()
	}
	return rec.baseline
}

// disconnect leaves the record with no connection and no stored value it knows.
func (rec *record) disconnect() {
	rec.stored, rec.baseline, rec.token, rec.connected = nil, nil, nil, false
}

// servable reports whether a shipped adapter serves the account's provider and the account has the
// OAuth client its provider authenticates through where it has one, and logs each account it is not
// (ADR-0010, ADR-0106). Which providers authenticate through a client is one of the facts the list of
// adapters holds, so the connections stay blind to which they are.
func (c *Connections) servable(account string, rec *record) bool {
	facts, ok := c.adapters.Facts(rec.provider)
	switch {
	case !ok:
		c.log.Warn("there is no adapter for an account's provider, so it is not connected", slog.String("account", account), slog.String("provider", rec.provider))
		return false
	case facts.UsesClient && !rec.hasClient:
		c.log.Warn("an account's provider authenticates through an OAuth client and the account has none that opened, so it is not connected",
			slog.String("account", account), slog.String("provider", rec.provider), slog.String("client", rec.clientName))
		return false
	}
	return true
}

// openClient opens a stored client's secret, bound to the client's row by its name. A secret that
// does not open is logged, and the client is not served.
func (c *Connections) openClient(cl oauthclients.OAuthClientsRow) (coremail.OAuthClient, bool) {
	secret, err := c.keys.Open(cl.SealedClientSecret, seal.ClientSecret(cl.ClientName))
	if err != nil {
		c.log.Error("an OAuth client's secret did not open, so no account connects through it", slog.String("client", cl.ClientName), slog.Any("error", err))
		return coremail.OAuthClient{}, false
	}
	return coremail.OAuthClient{ID: cl.ClientID, Secret: string(secret)}, true
}

// pair returns the client an account's row names, from the clients whose secrets opened, keyed on
// their names. An account that names none has none, and no client is looked up by provider (ADR-0106).
func pair(name string, opened map[string]coremail.OAuthClient) (coremail.OAuthClient, bool) {
	if name == "" {
		return coremail.OAuthClient{}, false
	}
	cl, ok := opened[name]
	return cl, ok
}

// onOldKey reports whether a stored value names a key other than the current one.
func (c *Connections) onOldKey(stored []byte) bool {
	parts, err := seal.Split(stored)
	return err != nil || parts.KeyID != c.keys.Current()
}

// readTarget reads the lowered target of a connected account into its record. A read that fails is
// that account's alone, logged and kept for its units of work to be refused with.
func (c *Connections) readTarget(ctx context.Context, account string, rec *record) {
	rec.mu.Lock()
	connected := rec.connected
	rec.mu.Unlock()
	if !connected {
		return
	}
	var fraction float64
	var lowered bool
	err := tx.Run(ctx, c.db, account, func(q pgx.Tx) error {
		read, err := accountstate.New(q).LoweredTarget(ctx)
		if err != nil {
			return err
		}
		fraction, lowered = float64(read.Float32), read.Valid
		return nil
	})
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.targetRead, rec.target, rec.lowered, rec.targetErr = true, fraction, lowered, nil
	if err != nil {
		rec.targetErr = fmt.Errorf("reading the lowered target of account %s: %w", account, err)
		c.log.Warn("reading an account's lowered target failed, so its work stops until a reload reads it", "account", account, "error", err)
	}
}

// sameClient reports whether two clients are the same identifier and secret.
func sameClient(a, b coremail.OAuthClient) bool {
	return a.ID == b.ID && a.Secret == b.Secret
}
