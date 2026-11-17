package connections

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients/secret"
)

// Scan is what the last load and re-seal found still sealed to a key other than the current one or
// unable to be opened, which a key's retirement waits on (ADR-0092). It holds an entry for every
// account the load listed and for every row of oauth_clients, so a missing entry never reads as done.
// An account with no stored credential reads false, since there is no value to seal.
type Scan struct {
	// Accounts is keyed on the account.
	Accounts map[string]bool
	// Clients is keyed on the client's name.
	Clients map[string]bool
}

func newScan() Scan { return Scan{Accounts: map[string]bool{}, Clients: map[string]bool{}} }

// clone returns a copy of the scan its caller may keep.
func (s Scan) clone() Scan {
	return Scan{Accounts: maps.Clone(s.Accounts), Clients: maps.Clone(s.Clients)}
}

// Reseal seals again to the current key every account credential and every OAuth client secret the
// last load opened with a key that is not the current one, writes each by compare-and-set on the
// bytes the connections knew, and returns the scan that results (ADR-0089, ADR-0092). Only delta
// sync's package calls it, which the go vet analyser rule on its callers holds (ADR-0071). A value
// someone else replaced meanwhile is read again and adopted, and not re-sealed until the next run. A
// write that fails is logged and leaves the value on its old key.
func (c *Connections) Reseal(ctx context.Context) Scan {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resealAccounts(ctx)
	c.resealClients(ctx)
	return c.scan.clone()
}

// resealAccounts re-seals the account credentials, as Reseal states, with the connections' lock held.
func (c *Connections) resealAccounts(ctx context.Context) {
	records := *c.records.Load()
	for _, account := range slices.Sorted(maps.Keys(records)) {
		c.resealAccount(ctx, account, records[account])
	}
}

// resealAccount re-seals one account's credential, as Reseal states, with the connections' lock
// held, so it takes turns with the account's write-backs (ADR-0089).
func (c *Connections) resealAccount(ctx context.Context, account string, rec *record) {
	rec.mu.Lock()
	known := rec.stored
	rec.mu.Unlock()
	if known == nil || !c.onOldKey(known) {
		return
	}
	_, resealed, changed, err := c.keys.Reseal(known, seal.AccountCredential(account))
	if err == nil && changed {
		var landed bool
		landed, err = c.replace(ctx, account, known, resealed)
		switch {
		case err != nil:
		case landed:
			rec.mu.Lock()
			rec.stored = resealed
			rec.mu.Unlock()
		default:
			stored, readErr := c.stored(ctx, account)
			if readErr == nil {
				rec.mu.Lock()
				c.readAgain(account, rec, stored)
				adopted := rec.stored != nil
				rec.mu.Unlock()
				c.scan.Accounts[account] = stored != nil && (!adopted || c.onOldKey(stored))
				return
			}
			err = readErr
		}
	}
	if err != nil {
		c.log.Error("an account's credential was not re-sealed to the current key", slog.String("account", account), slog.Any("error", err))
		return
	}
	rec.mu.Lock()
	if rec.stored != nil {
		c.scan.Accounts[account] = c.onOldKey(rec.stored)
	}
	rec.mu.Unlock()
}

// resealClients re-seals the OAuth client secrets, as Reseal states, with the connections' lock held.
// The write is the one statement that writes a client secret beside the UI's setup (ADR-0016).
func (c *Connections) resealClients(ctx context.Context) {
	for _, name := range slices.Sorted(maps.Keys(c.clients)) {
		known := c.clients[name]
		if !c.onOldKey(known) {
			continue
		}
		_, resealed, changed, err := c.keys.Reseal(known, seal.ClientSecret(name))
		if err == nil && changed {
			var landed bool
			if landed, err = c.replaceClient(ctx, name, known, resealed); err == nil && !landed {
				err = c.readClientAgain(ctx, name)
				if err == nil {
					continue
				}
			}
			if landed {
				c.clients[name] = resealed
			}
		}
		if err != nil {
			c.log.Error("an OAuth client's secret was not re-sealed to the current key", slog.String("client", name), slog.Any("error", err))
			continue
		}
		c.scan.Clients[name] = c.onOldKey(c.clients[name])
	}
}

// replaceClient writes the named client's secret sealed again, only if the stored bytes are still
// known, and reports whether the write landed (ADR-0089).
func (c *Connections) replaceClient(ctx context.Context, name string, known, sealed []byte) (bool, error) {
	var n int64
	err := pgx.BeginFunc(ctx, c.db, func(t pgx.Tx) error {
		var err error
		n, err = secret.New(t).ReplaceSealedClient(ctx, secret.ReplaceSealedClientParams{ClientSecret: sealed, ClientName: name, Known: known})
		return err
	})
	return n == 1, err
}

// readClientAgain reads the named client's stored secret again after someone else replaced it, opens
// it, pairs it with every account that names the client, a token made again from the new client
// included, and sets the client's scan entry from it, as a load would. A client whose row is gone is
// taken from its accounts with its entry, and one whose secret does not open is taken from its
// accounts and reads true (ADR-0089, ADR-0090, ADR-0106).
func (c *Connections) readClientAgain(ctx context.Context, name string) error {
	rows, err := c.readClients(ctx)
	if err != nil {
		return err
	}
	delete(c.clients, name)
	delete(c.opened, name)
	delete(c.scan.Clients, name)
	for _, cl := range rows {
		if cl.ClientName != name {
			continue
		}
		client, ok := c.openClient(cl)
		c.scan.Clients[name] = !ok || c.onOldKey(cl.SealedClientSecret)
		if ok {
			c.opened[name] = client
			c.clients[name] = cl.SealedClientSecret
		}
		break
	}
	records := *c.records.Load()
	for _, account := range slices.Sorted(maps.Keys(records)) {
		rec := records[account]
		rec.mu.Lock()
		if rec.clientName == name {
			c.repair(account, rec)
		}
		rec.mu.Unlock()
	}
	return nil
}

// repair pairs the record with the client it names as the connections now hold it, and makes its
// token again from the client it now pairs with when that changed. The caller holds the connections'
// lock and the record's.
func (c *Connections) repair(account string, rec *record) {
	client, hasClient := pair(rec.clientName, c.opened)
	changed := rec.hasClient != hasClient || !sameClient(rec.client, client)
	rec.client, rec.hasClient = client, hasClient
	if rec.stored == nil || !changed {
		return
	}
	if !c.servable(account, rec) {
		rec.token, rec.connected = nil, false
		return
	}
	token, err := c.adapters.NewToken(rec.provider, client, rec.current())
	if err != nil {
		c.log.Error("an account's token could not be made, so it is not connected", slog.String("account", account), slog.Any("error", err))
		rec.token, rec.connected = nil, false
		return
	}
	rec.token, rec.connected = token, true
}
