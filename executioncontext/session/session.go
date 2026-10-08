// Package session holds what a process calling a provider holds for an account while it calls the
// account's provider, the account session. It pairs the account with the credentials its snapshot
// opened, holds a token source and the port built over it, reads a refused credential again and makes
// the call once more over a replaced one, and at the end of a unit of work hands the source's
// credential over with its adoption stamp and records the latest authentication attempt (ADR-0089,
// ADR-0090, ADR-0097, ADR-0106). It imports no provider adapter and no rate limiter. Each composition
// root passes a connector per provider it links, and leases inside each call it makes. Its case is
// argued in executioncontext/README.md.
package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/authentication"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
)

// ErrNotConnected is an account the holder can open no session for, which is one its snapshot does
// not list, one that is not connected, or one whose provider has no connector.
var ErrNotConnected = errors.New("the account has no connected credential")

// Credentials are what a source is built from, as plain values. They are the identifier and secret
// of the OAuth client the account connects through, empty where its provider has none, and the
// account's credential, as the snapshot opened them.
type Credentials struct {
	ClientID     string
	ClientSecret []byte
	Credential   []byte
}

// same reports whether c and o are the same credentials.
func same(c, o Credentials) bool {
	return c.ClientID == o.ClientID && bytes.Equal(c.ClientSecret, o.ClientSecret) && bytes.Equal(c.Credential, o.Credential)
}

// Source is a provider's token source for one account, built by a connector.
type Source interface {
	// Credential returns the credential the source holds now, a rotated one once the provider
	// rotated it.
	Credential() []byte
	// LastAttempt returns the source's latest authentication attempt and its outcome, or the zero
	// value when it has made none (ADR-0097).
	LastAttempt() mail.AuthAttempt
}

// connection is a source a connector built and the function that builds a port over it.
type connection struct {
	source Source
	port   func(account string) (mail.Port[context.Context], error)
}

// Connector builds an account's source and the port over it for one provider. A composition root
// builds one per provider it links, through Connect.
type Connector struct {
	connect func(Credentials) connection
}

// Connect returns the connector that builds a source with source and a port over it with port. The
// port is only ever built over a source the same connector built.
func Connect[S Source](source func(Credentials) S, port func(account string, source S) (mail.Port[context.Context], error)) Connector {
	return Connector{connect: func(c Credentials) connection {
		s := source(c)
		return connection{source: s, port: func(account string) (mail.Port[context.Context], error) { return port(account, s) }}
	}}
}

// Config is what a holder is built from.
type Config struct {
	// Loader is the account loader its caller owns, so when snapshots reload stays the caller's
	// (ADR-0090). A refused credential is read again through it, and a credential handed over to it.
	Loader *accountload.Loader
	// DB is where the latest authentication attempt is recorded, under the caller's role.
	DB tx.Beginner
	// Logger logs each account a listing skips.
	Logger *slog.Logger
	// Connectors are the connectors of the providers the caller links, keyed on the provider's name
	// as accounts name it.
	Connectors map[string]Connector
	// Hold keeps an account's source across units of work while the stored credential is the one it
	// was built from or the one it rotated to, so an access token the provider issued serves the units
	// after it. Without it each unit builds a source of its own.
	Hold bool
}

// held is an account's source the holder keeps across units of work, and the credentials it was
// built from.
type held struct {
	built Credentials
	conn  connection
}

// Holder opens the sessions of the accounts a process serves. It is safe for concurrent units of
// work, for one account as for several.
type Holder struct {
	config Config
	mu     sync.Mutex
	held   map[string]held
}

// New returns a holder built from c.
func New(c Config) *Holder {
	return &Holder{config: c, held: map[string]held{}}
}

// credentials returns the credentials the account's source is built from, the client the account
// connects through, where it has one, and its credential as the loader opened them.
func credentials(a accountload.Account) Credentials {
	c := Credentials{Credential: a.Credential()}
	if client, ok := a.Client(); ok {
		c.ClientID, c.ClientSecret = client.ID(), client.Secret()
	}
	return c
}

// Accounts returns the accounts of snapshot the holder can open a session for, sorted. An account
// that is not connected, or whose provider has no connector, is logged and left out.
func (h *Holder) Accounts(snapshot *accountload.Snapshot) []string {
	var out []string
	for _, a := range snapshot.Accounts() {
		if !a.Connected() {
			h.config.Logger.Warn("an account is not connected, so it is skipped", "account", a.ID())
			continue
		}
		if _, ok := h.config.Connectors[a.Provider()]; !ok {
			h.config.Logger.Warn("there is no adapter for an account's provider, so it is skipped", "account", a.ID(), "provider", a.Provider())
			continue
		}
		out = append(out, a.ID())
	}
	slices.Sort(out)
	return out
}

// Open opens a unit of work's session for the account, with the credentials snapshot holds for it
// and the adoption stamp they came with (ADR-0089). With Hold, the account's held source is used
// while snapshot's credentials are the ones it was built from or carry the credential it now holds,
// a rotation it made, and is replaced otherwise. It returns ErrNotConnected for an account it can open
// no session for, and the connector's error when the port cannot be built.
func (h *Holder) Open(snapshot *accountload.Snapshot, account string) (*Session, error) {
	a, ok := snapshot.Account(account)
	if !ok || !a.Connected() {
		return nil, ErrNotConnected
	}
	connector, ok := h.config.Connectors[a.Provider()]
	if !ok {
		return nil, ErrNotConnected
	}
	creds := credentials(a)
	conn := h.source(account, connector, creds)
	port, err := conn.port(account)
	if err != nil {
		return nil, err
	}
	return &Session{h: h, account: account, connector: connector, built: creds, adoption: a.Adoption(), conn: conn, port: port}, nil
}

// source returns the account's source for creds, the one it holds when Hold is set and creds are the
// ones it was built from or carry the credential it now holds, and a new one otherwise.
func (h *Holder) source(account string, connector Connector, creds Credentials) connection {
	if !h.config.Hold {
		return connector.connect(creds)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	kept, ok := h.held[account]
	sameClient := kept.built.ClientID == creds.ClientID && bytes.Equal(kept.built.ClientSecret, creds.ClientSecret)
	if ok && sameClient && (bytes.Equal(kept.built.Credential, creds.Credential) || bytes.Equal(kept.conn.source.Credential(), creds.Credential)) {
		return kept.conn
	}
	conn := connector.connect(creds)
	h.held[account] = held{built: creds, conn: conn}
	return conn
}

// keep holds conn, built from creds, as the account's source for the units of work after this one,
// when Hold is set.
func (h *Holder) keep(account string, creds Credentials, conn connection) {
	if !h.config.Hold {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.held[account] = held{built: creds, conn: conn}
}

// Session is one unit of work's calls to an account's provider. Its calls are made one at a time.
type Session struct {
	h         *Holder
	account   string
	connector Connector
	// built are the credentials the session's source was built from.
	built Credentials
	// adoption is the adoption stamp of the credential the source was built from, the snapshot's when
	// the session opened or a re-read's once the session went on with the re-read credential, which
	// the hand-over names so a credential someone else replaced since is never put back (ADR-0089).
	adoption uint64
	conn     connection
	port     mail.Port[context.Context]
}

// Call makes call over the session's port. When the provider refuses the credential, the credential
// is read again from the account's row before the refusal is returned (ADR-0090). A row holding
// another credential, or the account moved to another client, has a source built from the pair the
// account now connects through (ADR-0106) and the port built again over it, kept for the session's
// later calls and its hand-over, and for later sessions when the holder holds sources, and call made
// once more over it, the lease inside call included. A row holding the refused credential, or none,
// returns the refusal, as does a port that cannot be built over the re-read credential, joined with
// the reason, which leaves the session holding the source and stamp it had.
func Call[T any](ctx context.Context, s *Session, call func(context.Context, mail.Port[context.Context]) (T, error)) (T, error) {
	out, err := call(ctx, s.port)
	if !errors.Is(err, mail.ErrAuthentication) {
		return out, err
	}
	reread, rerr := s.h.config.Loader.Reread(ctx, s.account)
	if rerr != nil {
		return out, errors.Join(err, fmt.Errorf("reading the refused credential again: %w", rerr))
	}
	refused := s.built
	refused.Credential = s.conn.source.Credential()
	creds := credentials(reread)
	if !reread.Connected() || same(creds, refused) {
		return out, err
	}
	conn := s.connector.connect(creds)
	port, cerr := conn.port(s.account)
	if cerr != nil {
		return out, errors.Join(err, cerr)
	}
	s.h.keep(s.account, creds, conn)
	s.built, s.adoption, s.conn, s.port = creds, reread.Adoption(), conn, port
	return call(ctx, port)
}

// End ends a unit of work over the session. It hands the source's current credential to the loader,
// naming the adoption stamp of the credential the source was built from, so the loader writes a
// rotated one back by compare-and-set and discards it when someone else replaced the credential since
// (ADR-0082, ADR-0089), and records the latest authentication attempt the source reports (ADR-0097).
// It returns the hand-over's error and the recording's apart, for the caller to return or log. A
// session may end several units of work, one after each.
func (s *Session) End(ctx context.Context) (handOver, record error) {
	_, handOver = s.h.config.Loader.HandOver(ctx, s.account, s.adoption, s.conn.source.Credential())
	return handOver, recordAttempt(ctx, s.h.config.DB, s.account, s.conn.source.LastAttempt())
}

// recordAttempt records the latest authentication attempt the account's source reported on the
// account's state row. The zero attempt, from a source that has made none, records nothing, and the
// statement keeps a later attempt another deployable recorded (ADR-0097). A recording that fails is
// returned, and the next unit of work records the attempt again.
func recordAttempt(ctx context.Context, db tx.Beginner, account string, attempt mail.AuthAttempt) error {
	if attempt == (mail.AuthAttempt{}) {
		return nil
	}
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		return authentication.New(t).RecordAuthentication(ctx, authentication.RecordAuthenticationParams{
			AttemptedAt: pgtype.Timestamptz{Time: time.UnixMilli(int64(attempt.At)).UTC(), Valid: true},
			Outcome:     pgtype.Text{String: string(attempt.Outcome), Valid: true},
			AccountID:   account,
		})
	})
	if err != nil {
		return fmt.Errorf("account %s: recording the last authentication attempt: %w", account, err)
	}
	return nil
}
