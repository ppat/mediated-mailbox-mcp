package accountload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// ErrNotConnected is returned for an account the loader holds no stored credential for, so there are
// no bytes a compare-and-set could compare against.
var ErrNotConnected = errors.New("the account has no stored credential the loader knows")

// Persist writes back the credential a provider adapter holds, such as the refresh token a Gmail
// token source holds, without naming the Adoption that came with the credential it was built from. A
// unit of work hands its credential over through HandOver instead, so a value someone else stored
// since that credential was taken is never put back. A credential equal to the one last read or
// written needs nothing. Any other is a rotation, sealed and written to the account's state row only
// if the row still holds the bytes the loader last read or wrote (ADR-0082, ADR-0089). It returns the
// credential the caller should hold from then on.
//
//   - When the write lands, that is the rotated credential.
//   - When the row holds other bytes, someone else replaced the value. The write changes nothing, and
//     the stored value is read again, opened and returned, so the caller drops its own.
//   - When the write fails, the rotated credential is held in memory, the failure is logged and
//     returned, and a reload keeps the rotated credential while the stored bytes stay the ones the
//     loader knew (ADR-0090). The next unit of work's hand-over writes it again. A restart before a
//     write lands loses it.
func (l *Loader) Persist(ctx context.Context, account string, current []byte) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.persist(ctx, account, current)
}

// HandOver is the call a deployable makes at the end of each unit of work. It is Persist for a unit
// that holds current as it ends and names adoption, the Adoption that came with the credential
// current was built from, which is the account's Adoption when that credential was taken, from the
// snapshot or from a re-read the unit went on with. When the adoption has moved, a reload or a
// re-read adopted a value someone else stored since that credential was taken, so current is stale
// beside it. It is discarded and the credential the loader holds is returned, since a compare-and-set
// against the adopted bytes would land and put the replaced value back (ADR-0089). A write-back of
// the loader's own moves nothing, so a rotation handed over by a unit that overlapped one whose
// rotation landed is written by compare-and-set against that rotation's bytes. The check and the
// write hold the same lock a reload and a re-read take, so neither can land between them.
func (l *Loader) HandOver(ctx context.Context, account string, adoption uint64, current []byte) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if h, ok := l.accounts[account]; ok && h.adoption != adoption {
		return slices.Clone(h.plaintext), nil
	}
	return l.persist(ctx, account, current)
}

// persist is Persist with l.mu held.
func (l *Loader) persist(ctx context.Context, account string, current []byte) ([]byte, error) {
	h, ok := l.accounts[account]
	if !ok {
		return nil, fmt.Errorf("account %s: %w", account, ErrNotConnected)
	}
	if len(current) == 0 {
		return nil, fmt.Errorf("account %s: the adapter holds an empty credential", account)
	}
	if slices.Equal(current, h.baseline) {
		return h.baseline, nil
	}
	h.plaintext = slices.Clone(current)
	l.hold(account, h)
	sealed, err := l.keys.Seal(current, seal.AccountCredential(account))
	if err != nil {
		return l.failed(account, current, err)
	}
	landed, err := l.replace(ctx, account, h.known, sealed)
	if err != nil {
		return l.failed(account, current, err)
	}
	if landed {
		h.known, h.baseline = sealed, h.plaintext
		l.hold(account, h)
		return h.plaintext, nil
	}
	stored, err := l.stored(ctx, account)
	if err != nil {
		return l.failed(account, current, err)
	}
	return l.reread(account, stored), nil
}

// failed logs a write-back that did not land and keeps the rotated credential held.
func (l *Loader) failed(account string, rotated []byte, err error) ([]byte, error) {
	l.log.Error("a rotated credential was not written back, so a restart before a later write-back lands loses the account's access",
		slog.String("account", account), slog.Any("error", err))
	return slices.Clone(rotated), fmt.Errorf("account %s: writing back the rotated credential: %w", account, err)
}

// replace runs the compare-and-set and reports whether it changed the row.
func (l *Loader) replace(ctx context.Context, account string, known, sealed []byte) (bool, error) {
	var changed int64
	err := tx.Run(ctx, l.db, account, func(t pgx.Tx) error {
		var err error
		changed, err = credential.New(t).ReplaceSealed(ctx, credential.ReplaceSealedParams{
			Credential: sealed, AccountID: account, Known: known,
		})
		return err
	})
	return changed == 1, err
}

// Reread reads the account's stored credential again, with the OAuth client its row names, as a
// deployable does when the provider refuses the credential it holds, before it reports the refusal
// (ADR-0090). It returns the account as the loader then holds it, whose credential is the one to use,
// the one the operator stored when it changed, paired with the client the account now connects
// through, since moving an account to another client writes the client and the credential together
// (ADR-0106). A unit of work that goes on with it builds its source from both and hands over its
// Adoption. An account no longer connected, or left without the client its provider authenticates
// through, is returned not connected.
func (l *Loader) Reread(ctx context.Context, account string) (Account, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	stored, err := l.stored(ctx, account)
	if err != nil {
		return Account{}, err
	}
	a, _ := l.active.Load().Account(account)
	a.id = account
	listed, clients, err := l.readClientOf(ctx, account)
	if err != nil {
		return Account{}, err
	}
	name := a.clientName
	if listed != nil {
		a.provider, name = listed.AccountProvider, listed.OauthClient.String
	}
	opened := map[string]Client{}
	for _, c := range clients {
		if c.ClientName == name {
			if client, ok := l.openClient(c); ok {
				opened[name] = client
			}
			break
		}
	}
	a = l.paired(a, name, opened)
	if l.reread(account, stored) == nil {
		a.credential, a.adoption = nil, 0
	} else {
		h := l.accounts[account]
		a.credential, a.adoption = h.plaintext, h.adoption
	}
	l.publishClient(a)
	return a, nil
}

// readClientOf reads the account's row of the listing and the stored OAuth clients, outside the
// account's transaction since neither belongs to it (ADR-0091). An account the listing no longer
// holds reads as nil.
func (l *Loader) readClientOf(ctx context.Context, account string) (*accounts.AccountsRow, []oauthclients.OAuthClientsRow, error) {
	var listed []accounts.AccountsRow
	var clients []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, l.db, func(t pgx.Tx) error {
		var err error
		if listed, err = accounts.New(t).Accounts(ctx); err != nil {
			return fmt.Errorf("listing the accounts again: %w", err)
		}
		if clients, err = oauthclients.New(t).OAuthClients(ctx); err != nil {
			return fmt.Errorf("reading the OAuth clients again: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for i := range listed {
		if listed[i].AccountID == account {
			return &listed[i], clients, nil
		}
	}
	return nil, clients, nil
}

// publishClient stores a snapshot equal to the active one but for the account's client.
func (l *Loader) publishClient(a Account) {
	current := l.active.Load()
	next := &Snapshot{accounts: slices.Clone(current.accounts)}
	for i := range next.accounts {
		if next.accounts[i].id == a.id {
			next.accounts[i].clientName, next.accounts[i].client = a.clientName, a.client
			next.accounts[i].hasClient, next.accounts[i].withoutClient = a.hasClient, a.withoutClient
		}
	}
	l.active.Store(next)
}

// reread adopts a value just read from the account's row and publishes it in a new snapshot.
func (l *Loader) reread(account string, stored []byte) []byte {
	h, ok := l.adopt(account, stored)
	if !ok {
		delete(l.accounts, account)
		l.publish(account, held{})
		return nil
	}
	l.hold(account, h)
	return h.plaintext
}

// hold records what the loader holds for an account and publishes its plaintext in a new snapshot, so
// a unit of work that starts later takes it. A unit already running keeps the snapshot it took.
func (l *Loader) hold(account string, h held) {
	l.accounts[account] = h
	l.publish(account, h)
}

// publish stores a snapshot equal to the active one but for the account's credential and adoption.
func (l *Loader) publish(account string, h held) {
	current := l.active.Load()
	next := &Snapshot{accounts: slices.Clone(current.accounts)}
	for i := range next.accounts {
		if next.accounts[i].id == account {
			next.accounts[i].credential, next.accounts[i].adoption = h.plaintext, h.adoption
		}
	}
	l.active.Store(next)
}
