package accountload

import (
	"context"
	"log/slog"
	"maps"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
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
// on its old key. It returns the scan that results. It re-seals account credentials only, since no
// statement this library runs writes an OAuth client's secret, so a client's scan entry reads true
// until its secret is re-sealed elsewhere.
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
