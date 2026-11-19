package connections

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
)

// Work is one unit of work on one account, one body request, one page of a pass or one tick. It is
// begun through the connections with the caller of the job kind or the operation that makes it, and
// ended once its calls are done. It carries no adoption stamp of its own. Its end reads the token's
// credential and the adoption stamp together from the record it began on (ADR-0089).
type Work struct {
	conns   *Connections
	account string
	record  *record
	mailbox mail.Mailbox
	// pool is the pool of the caller the unit began through, which its end writes back and records on
	// (ADR-0097, ADR-0119).
	pool tx.Beginner

	// mu guards the token the unit's mailbox calls with now and the client it was made from, which a
	// re-read after a refusal compares the account's row against.
	mu     sync.Mutex
	token  *provider.Token
	client coremail.OAuthClient
}

// Begin begins a unit of work on the account, calling through caller, whose leases may wait until
// until, the zero time for no bound (ADR-0130). It refuses with an error wrapping ErrNotConnected an
// account the Session's list in force does not hold or that has no connection, and with the read's
// error an account whose lowered target could not be read, so no unit of work begins on it while every
// other account's go on (ADR-0024, ADR-0090).
func (c *Connections) Begin(account string, caller *Caller, until time.Time) (*Work, error) {
	if _, listed := c.session.Listed(account); !listed {
		return nil, fmt.Errorf("account %s is not in the list in force: %w", account, ErrNotConnected)
	}
	rec, ok := (*c.records.Load())[account]
	if !ok {
		return nil, fmt.Errorf("account %s: %w", account, ErrNotConnected)
	}
	rec.mu.Lock()
	connected, token, client, name := rec.connected, rec.token, rec.client, rec.provider
	read, target, lowered, targetErr := rec.targetRead, rec.target, rec.lowered, rec.targetErr
	rec.mu.Unlock()
	switch {
	case !connected:
		return nil, fmt.Errorf("account %s: %w", account, ErrNotConnected)
	case !read:
		return nil, fmt.Errorf("no lowered target was read for account %s, since no reload found it connected", account)
	case targetErr != nil:
		return nil, targetErr
	}
	w := &Work{conns: c, account: account, record: rec, pool: caller.pool, token: token, client: client}
	m, err := caller.calls.Mailbox(provider.Connection{
		Provider: name, Account: account, Token: token, Target: target, Lowered: lowered, WaitUntil: until,
		Again: w.again,
	})
	if err != nil {
		return nil, err
	}
	w.mailbox = m
	return w, nil
}

// Mailbox returns the unit's mailbox, which every call of the unit goes through.
func (w *Work) Mailbox() mail.Mailbox { return w.mailbox }

// again reads the account's row once more after the provider refused the credential of the token the
// unit calls with, as the mailbox asks before the refusal stands (ADR-0090). It reads the stored
// credential and the OAuth client the row names now, and returns the token over that pair, or nil when
// the row holds the refused credential through the same client, or none, so the refusal stands.
//
// It changes the account's record in place, so the units of work after it, and those already running,
// follow it. A row holding the value the record's token already pairs with, made by another unit's
// re-read, gives that token. Another credential or another client gives a token made again from the
// pair (ADR-0106), which replaces the record's token together with the stored bytes it pairs with.
// When that token cannot be made the record keeps the token and the stored bytes it had, so a unit's
// write-back fails its compare-and-set against the row and the refused credential is never written
// back. An account the row leaves without a credential, or without the client its provider
// authenticates through, loses its connection.
func (w *Work) again(ctx context.Context) (*provider.Token, error) {
	c := w.conns
	c.mu.Lock()
	defer c.mu.Unlock()
	stored, err := c.stored(ctx, w.account)
	if err != nil {
		return nil, err
	}
	listed, rows, err := c.readClientOf(ctx, w.account)
	if err != nil {
		return nil, err
	}
	rec := w.record
	rec.mu.Lock()
	defer rec.mu.Unlock()
	name := rec.clientName
	if listed != nil {
		rec.provider, name = listed.AccountProvider, listed.OauthClient.String
	}
	opened := map[string]coremail.OAuthClient{}
	for _, cl := range rows {
		if cl.ClientName == name {
			if client, ok := c.openClient(cl); ok {
				opened[name] = client
			}
			break
		}
	}
	client, hasClient := pair(name, opened)
	followed := rec.hasClient == hasClient && sameClient(rec.client, client)
	rec.clientName, rec.client, rec.hasClient = name, client, hasClient
	if stored == nil {
		rec.disconnect()
		return nil, nil
	}
	same := rec.stored != nil && bytes.Equal(rec.stored, stored)
	plaintext := rec.current()
	if !same {
		if plaintext, err = c.keys.Open(stored, seal.AccountCredential(w.account)); err != nil {
			c.log.Error("an account's credential did not open, so it is not connected", slog.String("account", w.account), slog.Any("error", err))
			rec.disconnect()
			return nil, nil
		}
	}
	if !c.servable(w.account, rec) {
		rec.token, rec.connected = nil, false
		return nil, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if sameClient(client, w.client) && bytes.Equal(plaintext, w.token.Credential()) {
		return nil, nil
	}
	if same && followed && rec.token != nil && rec.token != w.token {
		w.token = rec.token
		return rec.token, nil
	}
	token, err := c.adapters.NewToken(rec.provider, client, plaintext)
	if err != nil {
		return nil, err
	}
	c.adopted(rec, same)
	rec.stored, rec.baseline, rec.token, rec.connected = stored, plaintext, token, true
	w.token, w.client = token, client
	return token, nil
}

// readClientOf reads the account's row of the listing and the stored OAuth clients, outside the
// account's transaction since neither belongs to it (ADR-0091). An account the listing no longer
// holds reads as nil.
func (c *Connections) readClientOf(ctx context.Context, account string) (*accounts.AccountsRow, []oauthclients.OAuthClientsRow, error) {
	var listed []accounts.AccountsRow
	var clients []oauthclients.OAuthClientsRow
	err := pgx.BeginFunc(ctx, c.db, func(t pgx.Tx) error {
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

// End ends the unit of work. It reads the credential the token of the record it began on holds now
// and the record's adoption stamp together, under the record's lock, and then writes the credential
// back by compare-and-set on the stored bytes, only while the record's stamp is still the one it
// read. So a unit whose end the connections adopted a value someone else stored under writes nothing
// back, and the process's own write-back, which leaves the stamp as it is, never stops another's
// (ADR-0082, ADR-0089). It then records the latest authentication attempt the token reports
// (ADR-0097). Both run on the pool of the caller the unit began through. It returns the write-back's
// error and the recording's apart, for the caller to return or log.
//
// A credential equal to the one last read or written needs no write. A rotated one is sealed and
// written only while the row holds the bytes the record knew. When the row holds other bytes, someone
// else replaced the value, the write changes nothing, and the stored value is read again and adopted
// while the stamp is still the one read. When the write fails, the failure is logged and returned,
// and the record keeps the rotated credential in its token, so the next unit's end writes it again
// and a reload keeps it while the stored bytes stay the same. A restart before a write lands loses
// it. The write-back holds the connections' lock from its stamp check through its compare-and-set and
// its re-read to the record's update, so it takes turns with every other write-back, every reload's
// read and adoption, every re-read after a refusal and the re-seal.
// Beginning a unit of work never waits on that lock, and its calls wait on it only for the re-read a
// refused call makes.
func (w *Work) End(ctx context.Context) (writeBack, record error) {
	rec := w.record
	rec.mu.Lock()
	token, stamp := rec.token, rec.stamp
	var current []byte
	if token != nil {
		current = token.Credential()
	}
	rec.mu.Unlock()
	if token == nil {
		return fmt.Errorf("account %s: %w", w.account, ErrNotConnected), nil
	}
	attempt := token.LastAttempt()
	return w.writeBack(ctx, stamp, current), recordAttempt(ctx, w.pool, w.account, attempt)
}

// writeBack writes current back, as End states, under the stamp End read with it.
func (w *Work) writeBack(ctx context.Context, stamp uint64, current []byte) error {
	c, rec, account := w.conns, w.record, w.account
	if len(current) == 0 {
		return fmt.Errorf("account %s: the adapter holds an empty credential", account)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	rec.mu.Lock()
	if rec.stamp != stamp {
		rec.mu.Unlock()
		return nil
	}
	known, baseline := rec.stored, rec.baseline
	rec.mu.Unlock()
	if bytes.Equal(current, baseline) {
		return nil
	}
	sealed, err := c.keys.Seal(current, seal.AccountCredential(account))
	if err != nil {
		return c.failed(account, err)
	}
	landed, err := replace(ctx, w.pool, account, known, sealed)
	if err != nil {
		return c.failed(account, err)
	}
	if landed {
		rec.mu.Lock()
		if rec.stamp == stamp && bytes.Equal(rec.stored, known) {
			rec.stored, rec.baseline = sealed, current
		}
		rec.mu.Unlock()
		return nil
	}
	read, err := storedOn(ctx, w.pool, account)
	if err != nil {
		return c.failed(account, err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.stamp == stamp {
		c.readAgain(account, rec, read)
	}
	return nil
}

// failed logs a write-back that did not land, without the credential, and returns its error.
func (c *Connections) failed(account string, err error) error {
	c.log.Error("a rotated credential was not written back, so a restart before a later write-back lands loses the account's access",
		slog.String("account", account), slog.Any("error", err))
	return fmt.Errorf("account %s: writing back the rotated credential: %w", account, err)
}

// replace runs the compare-and-set on the connections' pool and reports whether it changed the row.
func (c *Connections) replace(ctx context.Context, account string, known, sealed []byte) (bool, error) {
	return replace(ctx, c.db, account, known, sealed)
}

// replace runs the compare-and-set through db and reports whether it changed the row.
func replace(ctx context.Context, db tx.Beginner, account string, known, sealed []byte) (bool, error) {
	var changed int64
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		var err error
		changed, err = credential.New(t).ReplaceSealed(ctx, credential.ReplaceSealedParams{
			Credential: sealed, Known: known,
		})
		return err
	})
	return changed == 1, err
}

// readAgain adopts a value just read from the account's row, someone else's, into the record, with a
// new adoption stamp and a token made from it through the client the record holds. A row holding no credential, or one that
// does not open, leaves the account not connected. The caller holds the record's lock.
func (c *Connections) readAgain(account string, rec *record, stored []byte) {
	if stored == nil {
		rec.disconnect()
		return
	}
	if bytes.Equal(stored, rec.stored) {
		return
	}
	plaintext, err := c.keys.Open(stored, seal.AccountCredential(account))
	if err != nil {
		c.log.Error("an account's credential did not open, so it is not connected", slog.String("account", account), slog.Any("error", err))
		rec.disconnect()
		return
	}
	c.adopted(rec, false)
	rec.stored, rec.baseline = stored, plaintext
	if !c.servable(account, rec) {
		rec.token, rec.connected = nil, false
		return
	}
	token, err := c.adapters.NewToken(rec.provider, rec.client, plaintext)
	if err != nil {
		c.log.Error("an account's token could not be made, so it is not connected", slog.String("account", account), slog.Any("error", err))
		rec.token, rec.connected = nil, false
		return
	}
	rec.token, rec.connected = token, true
}

// recordAttempt records the latest authentication attempt the account's token reported on the
// account's state row. The zero attempt, from a token that has made none, records nothing, and the
// statement keeps a later attempt another program recorded (ADR-0097). A recording that fails is
// returned, and the next unit of work records the attempt again.
func recordAttempt(ctx context.Context, db tx.Beginner, account string, attempt coremail.AuthAttempt) error {
	if attempt == (coremail.AuthAttempt{}) {
		return nil
	}
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		return accountstate.New(t).RecordAuthentication(ctx, accountstate.RecordAuthenticationParams{
			AttemptedAt: pgtype.Timestamptz{Time: time.UnixMilli(int64(attempt.At)).UTC(), Valid: true},
			Outcome:     pgtype.Text{String: string(attempt.Outcome), Valid: true},
		})
	})
	if err != nil {
		return fmt.Errorf("account %s: recording the last authentication attempt: %w", account, err)
	}
	return nil
}
