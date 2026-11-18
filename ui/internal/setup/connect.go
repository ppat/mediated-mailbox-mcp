package setup

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	accountsetup "github.com/ppat/mediated-mailbox-mcp/db/accounts/setup"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	clientsetup "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/setup"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

type connectRequest struct {
	Account       string   `json:"account"`
	Client        string   `json:"client"`
	Mailbox       string   `json:"mailbox"`
	LoweredTarget *float64 `json:"lowered_target"`
}

type reauthorizeRequest struct {
	Mailbox *string `json:"mailbox"`
	Client  *string `json:"client"`
}

type finishRequest struct {
	Address string `json:"address"`
}

// attemptView is what a page shows of the session's attempt, which a reload restores. The attempt's
// state names it, so a tab tells when a newer attempt replaced its own (docs/UI.md section 8.12).
type attemptView struct {
	Attempt        string `json:"attempt"`
	Kind           string `json:"kind"`
	Account        string `json:"account"`
	Mailbox        string `json:"mailbox"`
	Client         string `json:"client"`
	ExpiresAt      string `json:"expires_at"`
	ConsentAddress string `json:"consent_address"`
}

type attemptAnswer struct {
	Attempt *attemptView `json:"attempt"`
}

type finishAnswer struct {
	Kind    string `json:"kind"`
	Account string `json:"account"`
	Mailbox string `json:"mailbox"`
	Client  string `json:"client"`
}

func connectType() schema.Type {
	return schema.Obj("Connect",
		schema.F("account", schema.Str()),
		schema.F("client", schema.Str()),
		schema.F("mailbox", schema.Str()),
		schema.F("lowered_target", schema.Null(schema.Num())),
	)
}

func reauthorizeType() schema.Type {
	return schema.Obj("Reauthorize",
		schema.F("mailbox", schema.Null(schema.Str())),
		schema.F("client", schema.Null(schema.Str())),
	)
}

func finishType() schema.Type {
	return schema.Obj("Finish", schema.F("address", schema.Str()))
}

func attemptAnswerType() schema.Type {
	return schema.Obj("AttemptAnswer", schema.F("attempt", schema.Null(schema.Obj("ConsentAttempt",
		schema.F("attempt", schema.Str()),
		schema.F("kind", schema.Str(setup.Connect, setup.Reauthorize, setup.Move)),
		schema.F("account", schema.Str()),
		schema.F("mailbox", schema.Str()),
		schema.F("client", schema.Str()),
		schema.F("expires_at", schema.Time()),
		schema.F("consent_address", schema.Str()),
	))))
}

func finishAnswerType() schema.Type {
	return schema.Obj("Connected",
		schema.F("kind", schema.Str(setup.Connect, setup.Reauthorize, setup.Move)),
		schema.F("account", schema.Str()),
		schema.F("mailbox", schema.Str()),
		schema.F("client", schema.Str()),
	)
}

func viewOf(a attempt) *attemptView {
	return &attemptView{
		Attempt: a.State, Kind: a.Kind, Account: a.Account, Mailbox: a.Mailbox, Client: a.Client,
		ExpiresAt: registry.Stamp(time.UnixMilli(a.Expires)), ConsentAddress: a.Consent,
	}
}

// startConnect starts a consent attempt for the session through the named client, after checking the
// identifier, replacing any attempt the session held (docs/UI.md sections 8.12 and 17.4). It answers
// with the consent page's address, which the page opens in a new tab.
func (s *handlers) startConnect(w http.ResponseWriter, r *http.Request) {
	var body connectRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	if refused := cmp.Or(setup.Identifier(body.Account), setup.Mailbox(body.Mailbox), setup.Target(body.LoweredTarget)); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	listed, err := s.listed(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	if refused := setup.Unclaimed(listed, body.Account); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	client, ok := s.client(w, r, body.Client)
	if !ok {
		return
	}
	s.start(w, r, attempt{
		Kind: setup.Connect, Account: body.Account, Provider: client.Provider, Mailbox: body.Mailbox,
		Client: client.Name, ClientID: client.ID, LoweredTarget: body.LoweredTarget,
	})
}

// startReauthorize starts a consent attempt re-authorizing the path's account, through its own client
// or, to move it, another client of its provider. The grant is checked against the mailbox the account
// remembers, and a request naming a mailbox of its own is refused while one is remembered (ADR-0080).
func (s *handlers) startReauthorize(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	var body reauthorizeRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	row, err := s.accountSetup(r.Context(), account)
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	held := setup.Held{Provider: row.AccountProvider, Client: row.OauthClient.String, Mailbox: registry.OptionalText(row.Mailbox)}
	name, refused := setup.ClientOf(held, body.Client)
	if refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	client, ok := s.client(w, r, name)
	if !ok {
		return
	}
	kind, mailbox, refused := setup.Reauthorization(held, client, body.Mailbox)
	if refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	s.start(w, r, attempt{
		Kind: kind, Account: account, Provider: held.Provider, Mailbox: mailbox, Remembered: held.Mailbox != nil,
		Client: client.Name, ClientID: client.ID,
	})
}

// client reads the named client's identity, and answers the request when no client holds the name
// or its provider connects through no consent the server holds.
func (s *handlers) client(w http.ResponseWriter, r *http.Request, name string) (setup.Client, bool) {
	stored, err := s.clients(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return setup.Client{}, false
	}
	client, refused := setup.Named(stored, name, slices.Collect(maps.Keys(s.opts.Consents)))
	if refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return setup.Client{}, false
	}
	return client, true
}

// clients reads every stored client's identity, as the setup core decides against it.
func (s *handlers) clients(ctx context.Context) ([]setup.Client, error) {
	rows, err := clientsetup.New(s.opts.Database).SetupClients(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]setup.Client, len(rows))
	for i, c := range rows {
		out[i] = setup.Client{Name: c.ClientName, Provider: c.Provider, ID: c.ClientID}
	}
	return out, nil
}

// listed reads every account's identifier, the accounts listing, which belongs to no account
// (ADR-0091).
func (s *handlers) listed(ctx context.Context) ([]string, error) {
	rows, err := accounts.New(s.opts.Database).Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, a := range rows {
		out[i] = a.AccountID
	}
	return out, nil
}

// accountSetup reads what account setup reads of an account, in a transaction set to it.
func (s *handlers) accountSetup(ctx context.Context, account string) (accountsetup.AccountSetupRow, error) {
	var held accountsetup.AccountSetupRow
	err := tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		rows, err := accountsetup.New(t).AccountSetup(ctx)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return errors.New("the account is not listed")
		}
		held = rows[0]
		return nil
	})
	return held, err
}

// start seals a new attempt into the session's cookie, replacing the one it held, and answers with
// what the page shows of it.
func (s *handlers) start(w http.ResponseWriter, r *http.Request, a attempt) {
	a.State, a.Verifier = rand.Text(), rand.Text()+rand.Text()
	a.Expires = s.opts.Clock().Add(attemptLifetime).UnixMilli()
	a.Consent = s.opts.Consents[a.Provider].ConsentAddress(a.ClientID, a.State, a.Verifier, a.Mailbox)
	sealed, err := s.keys.sealAttempt(a, server.SessionID(r.Context()))
	if err != nil {
		server.UIFailure(w, r, err)
		return
	}
	keepAttempt(w, sealed)
	server.WriteJSON(w, r, attemptAnswer{Attempt: viewOf(a)})
}

// getConnect reads the session's attempt to connect an account, which a reload of the page restores.
func (s *handlers) getConnect(w http.ResponseWriter, r *http.Request) {
	s.showAttempt(w, r, func(a attempt) bool { return a.Kind == setup.Connect })
}

// getReauthorize reads the session's attempt to re-authorize or move the path's account.
func (s *handlers) getReauthorize(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	s.showAttempt(w, r, func(a attempt) bool { return a.Kind != setup.Connect && a.Account == account })
}

func (s *handlers) showAttempt(w http.ResponseWriter, r *http.Request, mine func(attempt) bool) {
	a, err := s.keys.openAttempt(r)
	if err != nil || !mine(a) {
		server.WriteJSON(w, r, attemptAnswer{})
		return
	}
	server.WriteJSON(w, r, attemptAnswer{Attempt: viewOf(a)})
}

// finishConnect finishes the session's attempt to connect an account and writes its two rows.
func (s *handlers) finishConnect(w http.ResponseWriter, r *http.Request) {
	s.finish(w, r, func(a attempt) bool { return a.Kind == setup.Connect })
}

// finishReauthorize finishes the session's attempt to re-authorize or move the path's account.
func (s *handlers) finishReauthorize(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	s.finish(w, r, func(a attempt) bool { return a.Kind != setup.Connect && a.Account == account })
}

// errClientChanged is a client replaced or removed since the attempt started.
var errClientChanged = errors.New("the attempt's client was replaced or removed")

// client is the identity of the client the attempt started through.
func (a attempt) client() setup.Client {
	return setup.Client{Name: a.Client, Provider: a.Provider, ID: a.ClientID}
}

// finish finishes the session's attempt from the pasted address (docs/UI.md section 8.12). Each
// refusal stores nothing and leaves the attempt in place, so the operator opens the consent page
// again. A success writes everything the attempt was for in one transaction set to the account,
// records the code exchange's authentication attempt as the account's latest (ADR-0097), and ends
// the attempt.
func (s *handlers) finish(w http.ResponseWriter, r *http.Request, mine func(attempt) bool) {
	var body finishRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	a, err := s.keys.openAttempt(r)
	consent, known := s.opts.Consents[a.Provider]
	if err != nil || !mine(a) || !known {
		server.WriteFailure(w, r, server.ClientFault(refusal.NoAttempt, "no connection is in progress in this browser session"))
		return
	}
	if refused := setup.Expired(a.Expires, s.opts.Clock().UnixMilli()); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	redirect, err := consent.ReadRedirect(body.Address)
	if err != nil {
		server.RequestLogOf(r.Context()).Err = err
		server.WriteFailure(w, r, server.ClientFault(refusal.WrongAddress, "this is not the address the provider redirected to"))
		return
	}
	if refused := setup.Redirect(redirect, a.State); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return
	}
	if !s.stillFree(w, r, a) {
		return
	}
	secret, err := s.opts.Secret(r.Context(), a.Client)
	if err != nil {
		server.UIFailure(w, r, err)
		return
	}
	started := s.opts.Clock()
	grant, err := consent.Finish(r.Context(), mail.OAuthClient{ID: a.ClientID, Secret: secret}, redirect.Code, a.Verifier)
	if err != nil {
		s.consentFailure(w, r, a, err)
		return
	}
	if !consent.SameMailbox(grant.Mailbox, a.Mailbox) {
		server.WriteFailure(w, r, server.ClientFault(refusal.WrongMailbox, "Google granted access to "+grant.Mailbox+", not "+a.Mailbox+"."))
		return
	}
	if !a.Remembered {
		a.Mailbox = grant.Mailbox
	}
	sealed, err := s.opts.Seal.Seal([]byte(grant.Credential), seal.AccountCredential(a.Account))
	if err != nil {
		server.UIFailure(w, r, err)
		return
	}
	err = tx.Run(r.Context(), s.opts.Database, a.Account, func(t pgx.Tx) error {
		return s.store(r.Context(), clientsetup.New(t), accountsetup.New(t), accountstate.New(t), a, sealed, started)
	})
	if s.storeFailure(w, r, err) {
		return
	}
	endAttempt(w)
	server.WriteJSON(w, r, finishAnswer{Kind: a.Kind, Account: a.Account, Mailbox: a.Mailbox, Client: a.Client})
}

// stillFree refuses, before the code is exchanged, an identifier an account took since a connection's
// attempt started and a client replaced or removed since any attempt started. The transaction that
// writes checks both again.
func (s *handlers) stillFree(w http.ResponseWriter, r *http.Request, a attempt) bool {
	var listed []string
	if a.Kind == setup.Connect {
		var err error
		if listed, err = s.listed(r.Context()); err != nil {
			server.DatabaseFailure(w, r, err)
			return false
		}
	}
	stored, err := s.clients(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return false
	}
	if refused := setup.StillFree(a.Kind, a.Account, listed, stored, a.client()); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return false
	}
	return true
}

// consentFailure answers a code exchange or mailbox read the provider refused or did not answer.
func (s *handlers) consentFailure(w http.ResponseWriter, r *http.Request, a attempt, err error) {
	server.RequestLogOf(r.Context()).Err = err
	switch {
	case errors.Is(err, mail.ErrCodeRefused):
		server.WriteFailure(w, r, server.ClientFault(refusal.CodeRefused, "the provider refused the code"))
	case errors.Is(err, mail.ErrScopeMissing):
		server.WriteFailure(w, r, server.ClientFault(refusal.ScopeMissing, "the provider granted no access to the mailbox"))
	case errors.Is(err, mail.ErrScopeRefused):
		server.WriteFailure(w, r, server.ClientFault(refusal.ScopeRefused, "the provider granted more than the one permission asked for"))
	case errors.Is(err, mail.ErrAPIDisabled):
		server.WriteFailure(w, r, server.ClientFault(refusal.APIDisabled, "the provider's API is not enabled in the project of the client "+a.Client))
	default:
		s.providerFailure(w, r, err)
	}
}

// store writes what the attempt was for, in the transaction set to its account. It reads the client
// again first, so a client replaced or removed since the attempt started writes nothing. A connection
// writes the account's listing row and its state row together (ADR-0091). A re-authorization replaces
// the credential, or writes the state row of an account that has none, with the mailbox the account
// remembers from then on (ADR-0080). A move writes the new client and the credential issued to it
// together (ADR-0106). Each records the code exchange's attempt through the latest-wins statement
// (ADR-0097).
func (s *handlers) store(ctx context.Context, clients *clientsetup.Queries, q *accountsetup.Queries, auth *accountstate.Queries, a attempt, sealed []byte, started time.Time) error {
	identity, err := clients.ClientIdentity(ctx, a.Client)
	if err != nil {
		return err
	}
	read := make([]setup.Client, len(identity))
	for i, c := range identity {
		read[i] = setup.Client{Name: a.Client, Provider: c.Provider, ID: c.ClientID}
	}
	if setup.ClientChanged(read, a.client()) {
		return errClientChanged
	}
	client := pgtype.Text{String: a.Client, Valid: true}
	mailbox := pgtype.Text{String: a.Mailbox, Valid: true}
	switch a.Kind {
	case setup.Connect:
		if err := q.AddAccount(ctx, accountsetup.AddAccountParams{Provider: a.Provider, OauthClient: client}); err != nil {
			return err
		}
		target := pgtype.Float4{}
		if a.LoweredTarget != nil {
			target = pgtype.Float4{Float32: float32(*a.LoweredTarget), Valid: true}
		}
		if err := q.AddState(ctx, accountsetup.AddStateParams{Credential: sealed, Mailbox: mailbox, LoweredTarget: target}); err != nil {
			return err
		}
	default:
		if a.Kind == setup.Move {
			n, err := q.MoveAccount(ctx, accountsetup.MoveAccountParams{OauthClient: client, Provider: a.Provider})
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("the account to move is not listed")
			}
		}
		n, err := q.Reauthorize(ctx, accountsetup.ReauthorizeParams{Credential: sealed, Mailbox: a.Mailbox})
		if err != nil {
			return err
		}
		if n == 0 {
			if err := q.AddState(ctx, accountsetup.AddStateParams{Credential: sealed, Mailbox: mailbox}); err != nil {
				return err
			}
		}
	}
	return auth.RecordAuthentication(ctx, accountstate.RecordAuthenticationParams{
		AttemptedAt: pgtype.Timestamptz{Time: started, Valid: true},
		Outcome:     pgtype.Text{String: string(mail.AuthSucceeded), Valid: true},
	})
}

// storeFailure answers a transaction that stored nothing, and reports whether it did.
func (s *handlers) storeFailure(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	server.RequestLogOf(r.Context()).Err = err
	key, unique := server.Violated(err, server.UniqueViolation)
	_, check := server.Violated(err, server.CheckViolation)
	if refused := setup.Stored(errors.Is(err, errClientChanged), setup.Violation{Constraint: key, Unique: unique, Check: check}); refused != nil {
		server.WriteFailure(w, r, server.Refused(refused))
		return true
	}
	server.DatabaseFailure(w, r, err)
	return true
}
