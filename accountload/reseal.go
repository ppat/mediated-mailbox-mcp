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
	// Clients is keyed on the client's name.
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

// ClientWriter writes the secret of the client with the name sealed again, only if the stored bytes
// are still known, and reports whether the write landed (ADR-0089). Delta sync supplies it, through
// the one statement that writes a client secret, which only its role is granted (ADR-0092,
// ADR-0016).
type ClientWriter func(ctx context.Context, name string, known, sealed []byte) (bool, error)

// ResealClients seals every OAuth client secret the last load opened with a key that is not the
// current one again to the current key, and writes it through write by compare-and-set on the bytes
// the loader knew (ADR-0089, ADR-0092). Delta sync is the one process that calls it. A secret someone
// else replaced meanwhile is read again, opened and published in a new snapshot, and not re-sealed
// until the next run. A write that fails is logged and leaves the secret on its old key. It returns
// the scan that results.
func (l *Loader) ResealClients(ctx context.Context, write ClientWriter) Scan {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, name := range slices.Sorted(maps.Keys(l.clients)) {
		known := l.clients[name]
		if !l.onOldKey(known) {
			continue
		}
		_, resealed, changed, err := l.keys.Reseal(known, seal.ClientSecret(name))
		if err == nil && changed {
			var landed bool
			if landed, err = write(ctx, name, known, resealed); err == nil && !landed {
				err = l.rereadClient(ctx, name)
				if err == nil {
					continue
				}
			}
			if landed {
				l.clients[name] = resealed
			}
		}
		if err != nil {
			l.log.Error("an OAuth client's secret was not re-sealed to the current key", slog.String("client", name), slog.Any("error", err))
			continue
		}
		l.scan.Clients[name] = l.onOldKey(l.clients[name])
	}
	return Scan{Accounts: maps.Clone(l.scan.Accounts), Clients: maps.Clone(l.scan.Clients)}
}

// rereadClient reads the named client's stored secret again after someone else replaced it, opens it,
// publishes it in a new snapshot to every account that names the client and sets the client's scan
// entry from it, as a load would. A client whose row is gone is taken from its accounts with its
// entry, and one whose secret does not open is taken from its accounts and reads true (ADR-0089,
// ADR-0090, ADR-0106).
func (l *Loader) rereadClient(ctx context.Context, name string) error {
	var rows []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, l.db, func(t pgx.Tx) error {
		var err error
		rows, err = oauthclients.New(t).OAuthClients(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("reading the OAuth clients again: %w", err)
	}
	delete(l.clients, name)
	delete(l.scan.Clients, name)
	opened := map[string]Client{}
	for _, c := range rows {
		if c.ClientName != name {
			continue
		}
		client, ok := l.openClient(c)
		l.scan.Clients[name] = !ok || l.onOldKey(c.SealedClientSecret)
		if ok {
			opened[name] = client
			l.clients[name] = c.SealedClientSecret
		}
		break
	}
	next := &Snapshot{accounts: slices.Clone(l.active.Load().accounts)}
	for i, a := range next.accounts {
		if a.clientName == name {
			next.accounts[i] = l.paired(a, name, opened)
		}
	}
	l.active.Store(next)
	return nil
}
