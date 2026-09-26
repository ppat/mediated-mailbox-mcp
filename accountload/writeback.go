package accountload

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// ErrNotConnected is returned for an account the loader holds no stored credential for, so there are
// no bytes a compare-and-set could compare against.
var ErrNotConnected = errors.New("the account has no stored credential the loader knows")

// Persist is the call a deployable makes at the end of each unit of work with the credential its
// provider adapter holds at that moment, such as the refresh token a Gmail token source holds. A
// credential equal to the one last read or written needs nothing. Any other is a rotation, sealed
// and written to the account's state row only if the row still holds the bytes the loader last read
// or wrote (ADR-0082, ADR-0089). It returns the credential the caller should hold from then on.
//
//   - When the write lands, that is the rotated credential.
//   - When the row holds other bytes, someone else replaced the value. The write changes nothing, and
//     the stored value is read again, opened and returned, so the caller drops its own.
//   - When the write fails, the rotated credential is held in memory, the failure is logged and
//     returned, and a reload keeps the rotated credential while the stored bytes stay the ones the
//     loader knew (ADR-0090). The next unit of work's call writes it again. A restart before a write
//     lands loses it.
func (l *Loader) Persist(ctx context.Context, account string, current []byte) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
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
		changed, err = accountstate.New(t).ReplaceSealed(ctx, accountstate.ReplaceSealedParams{
			Credential: sealed, AccountID: account, Known: known,
		})
		return err
	})
	return changed == 1, err
}

// Reread reads the account's stored credential again, as a deployable does when the provider refuses
// the credential it holds, before it reports the refusal (ADR-0090). It returns the credential to use,
// the one the operator stored when it changed, or nil when the account is no longer connected.
func (l *Loader) Reread(ctx context.Context, account string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	stored, err := l.stored(ctx, account)
	if err != nil {
		return nil, err
	}
	return l.reread(account, stored), nil
}

// reread adopts a value just read from the account's row and publishes it in a new snapshot.
func (l *Loader) reread(account string, stored []byte) []byte {
	h, ok := l.adopt(account, stored)
	if !ok {
		delete(l.accounts, account)
		l.publish(account, nil)
		return nil
	}
	l.hold(account, h)
	return h.plaintext
}

// hold records what the loader holds for an account and publishes its plaintext in a new snapshot, so
// a unit of work that starts later takes it. A unit already running keeps the snapshot it took.
func (l *Loader) hold(account string, h held) {
	l.accounts[account] = h
	l.publish(account, h.plaintext)
}

// publish stores a snapshot equal to the active one but for the account's credential.
func (l *Loader) publish(account string, credential []byte) {
	current := l.active.Load()
	next := &Snapshot{accounts: slices.Clone(current.accounts), clients: current.clients}
	for i := range next.accounts {
		if next.accounts[i].id == account {
			next.accounts[i].credential = credential
		}
	}
	l.active.Store(next)
}
