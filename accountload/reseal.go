package accountload

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
)

// Scan is what the last load and re-seal found still sealed to a key other than the current one or
// unable to be opened (ADR-0092). It holds an entry for every account the load listed and for every
// row of oauth_clients, so a missing entry never reads as done. An account with no stored credential
// reads false, since there is no value to seal.
type Scan struct {
	// Accounts is keyed on the account.
	Accounts map[string]bool
	// Clients is keyed on the provider.
	Clients map[string]bool
}

func newScan() Scan { return Scan{Accounts: map[string]bool{}, Clients: map[string]bool{}} }

// Scan returns what the last load and re-seal found.
func (l *Loader) Scan() Scan {
	l.mu.Lock()
	defer l.mu.Unlock()
	return Scan{Accounts: maps.Clone(l.scan.Accounts), Clients: maps.Clone(l.scan.Clients)}
}

// Reseal seals every account credential the last load opened with a key that is not the current one
// again to the current key, and writes it by compare-and-set on the bytes the loader knew (ADR-0089,
// ADR-0092). Delta sync is the one process that calls it. A value someone else replaced meanwhile is
// read again and not re-sealed until the next run. A write that fails is logged and leaves the value
// on its old key. It returns the scan that results. It re-seals account credentials only, and
// ResealClients re-seals the OAuth clients' secrets through the write its caller supplies, since no
// statement this library runs writes a client secret.
func (l *Loader) Reseal(ctx context.Context) Scan {
	l.mu.Lock()
	defer l.mu.Unlock()
	for account, h := range l.accounts {
		if !l.onOldKey(h.known) {
			continue
		}
		_, resealed, changed, err := l.keys.Reseal(h.known, seal.AccountCredential(account))
		if err == nil && changed {
			var landed bool
			landed, err = l.replace(ctx, account, h.known, resealed)
			switch {
			case err != nil:
			case landed:
				h.known = resealed
				l.accounts[account] = h
			default:
				stored, readErr := l.stored(ctx, account)
				if readErr == nil {
					l.reread(account, stored)
					_, adopted := l.accounts[account]
					l.scan.Accounts[account] = stored != nil && (!adopted || l.onOldKey(stored))
					continue
				}
				err = readErr
			}
		}
		if err != nil {
			l.log.Error("an account's credential was not re-sealed to the current key", slog.String("account", account), slog.Any("error", err))
			continue
		}
		if h, ok := l.accounts[account]; ok {
			l.scan.Accounts[account] = l.onOldKey(h.known)
		}
	}
	return Scan{Accounts: maps.Clone(l.scan.Accounts), Clients: maps.Clone(l.scan.Clients)}
}

// ClientWriter writes a provider's client secret sealed again, only if the stored bytes are still
// known, and reports whether the write landed (ADR-0089). Delta sync supplies it, through the one
// statement that writes a client secret, which only its role is granted (ADR-0092, ADR-0016).
type ClientWriter func(ctx context.Context, provider string, known, sealed []byte) (bool, error)

// ResealClients seals every OAuth client secret the last load opened with a key that is not the
// current one again to the current key, and writes it through write by compare-and-set on the bytes
// the loader knew (ADR-0089, ADR-0092). Delta sync is the one process that calls it. A secret someone
// else replaced meanwhile is read again, opened and published in a new snapshot, and not re-sealed
// until the next run. A write that fails is logged and leaves the secret on its old key. It returns
// the scan that results.
func (l *Loader) ResealClients(ctx context.Context, write ClientWriter) Scan {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, provider := range slices.Sorted(maps.Keys(l.clients)) {
		known := l.clients[provider]
		if !l.onOldKey(known) {
			continue
		}
		_, resealed, changed, err := l.keys.Reseal(known, seal.ClientSecret(provider))
		if err == nil && changed {
			var landed bool
			if landed, err = write(ctx, provider, known, resealed); err == nil && !landed {
				err = l.rereadClient(ctx, provider)
				if err == nil {
					continue
				}
			}
			if landed {
				l.clients[provider] = resealed
			}
		}
		if err != nil {
			l.log.Error("an OAuth client's secret was not re-sealed to the current key", slog.String("provider", provider), slog.Any("error", err))
			continue
		}
		l.scan.Clients[provider] = l.onOldKey(l.clients[provider])
	}
	return Scan{Accounts: maps.Clone(l.scan.Accounts), Clients: maps.Clone(l.scan.Clients)}
}

// rereadClient reads the provider's stored client secret again after someone else replaced it, opens
// it, publishes it in a new snapshot and sets the provider's scan entry from it, as a load would. A
// provider whose row is gone loses its client and its entry, and one whose secret does not open loses
// its client and reads true (ADR-0089, ADR-0090).
func (l *Loader) rereadClient(ctx context.Context, provider string) error {
	var rows []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, l.db, func(t pgx.Tx) error {
		var err error
		rows, err = oauthclients.New(t).OAuthClients(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("reading the OAuth clients again: %w", err)
	}
	current := l.active.Load()
	next := &Snapshot{accounts: slices.Clone(current.accounts), clients: maps.Clone(current.clients)}
	delete(next.clients, provider)
	delete(l.clients, provider)
	delete(l.scan.Clients, provider)
	for _, c := range rows {
		if c.Provider != provider {
			continue
		}
		secret, err := l.keys.Open(c.SealedClientSecret, seal.ClientSecret(c.Provider))
		l.scan.Clients[provider] = err != nil || l.onOldKey(c.SealedClientSecret)
		if err != nil {
			l.log.Error("an OAuth client's secret did not open, so its provider has no client", slog.String("provider", provider), slog.Any("error", err))
			break
		}
		next.clients[provider] = Client{provider: provider, id: c.ClientID, secret: secret}
		l.clients[provider] = c.SealedClientSecret
	}
	l.active.Store(next)
	return nil
}
