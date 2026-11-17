package setup

import (
	"context"
	"crypto/rand"
	"errors"
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
		schema.F("kind", schema.Str(attemptConnect, attemptReauthorize, attemptMove)),
		schema.F("account", schema.Str()),
		schema.F("mailbox", schema.Str()),
		schema.F("client", schema.Str()),
		schema.F("expires_at", schema.Time()),
		schema.F("consent_address", schema.Str()),
	))))
}

func finishAnswerType() schema.Type {
	return schema.Obj("Connected",
		schema.F("kind", schema.Str(attemptConnect, attemptReauthorize, attemptMove)),
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

// identifierGrammar states the grammar an account identifier keeps, which a refusal names
// (docs/UI.md section 8.12).
const identifierGrammar = "an account identifier is 1 to 63 lowercase letters, digits and hyphens, starting and ending with a letter or a digit"

// startConnect starts a consent attempt for the session through the named client, after checking the
// identifier, replacing any attempt the session held (docs/UI.md sections 8.12 and 17.4). It answers
// with the consent page's address, which the page opens in a new tab.
func (s *handlers) startConnect(w http.ResponseWriter, r *http.Request) {
	var body connectRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	if refusal := setup.Identifier(body.Account); refusal != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(refusal), identifierGrammar))
		return
	}
	if !s.validStart(w, r, body.Mailbox, body.LoweredTarget) {
		return
	}
	listed, err := accounts.New(s.opts.Database).Accounts(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	if slices.ContainsFunc(listed, func(a accounts.AccountsRow) bool { return a.AccountID == body.Account }) {
		server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "identifier_taken", "an account already holds this identifier"))
		return
	}
	client, ok := s.client(w, r, body.Client)
	if !ok {
		return
	}
	s.start(w, r, attempt{
		Kind: attemptConnect, Account: body.Account, Provider: client.Provider, Mailbox: body.Mailbox,
		Client: client.ClientName, ClientID: client.ClientID, LoweredTarget: body.LoweredTarget,
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
	held, err := s.accountSetup(r.Context(), account)
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	name := held.OauthClient.String
	if body.Client != nil {
		name = *body.Client
	}
	if name == "" {
		server.WriteFailure(w, r, server.ClientFault(http.StatusNotFound, "unknown_client", "the account connects through no client"))
		return
	}
	client, ok := s.client(w, r, name)
	if !ok {
		return
	}
	if client.Provider != held.AccountProvider {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "client_wrong_provider", "the client belongs to another provider than the account's"))
		return
	}
	mailbox := held.Mailbox.String
	switch {
	case held.Mailbox.Valid && body.Mailbox != nil:
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "mailbox_remembered", "the account remembers its mailbox, so the request names none"))
		return
	case !held.Mailbox.Valid && body.Mailbox == nil:
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "mailbox_required", "the account remembers no mailbox, so the request names it"))
		return
	case !held.Mailbox.Valid:
		mailbox = *body.Mailbox
		if !s.validStart(w, r, mailbox, nil) {
			return
		}
	}
	kind := attemptReauthorize
	if client.ClientName != held.OauthClient.String {
		kind = attemptMove
	}
	s.start(w, r, attempt{
		Kind: kind, Account: account, Provider: held.AccountProvider, Mailbox: mailbox, Remembered: held.Mailbox.Valid,
		Client: client.ClientName, ClientID: client.ClientID,
	})
}

// validStart refuses a mailbox with no local part or domain and a lowered target outside its range.
func (s *handlers) validStart(w http.ResponseWriter, r *http.Request, mailbox string, target *float64) bool {
	if setup.Mailbox(mailbox) != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(setup.MailboxRefused), "the mailbox is an address with a local part and a domain"))
		return false
	}
	if setup.Target(target) != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(setup.TargetRefused), "a lowered target is above 5% and at most 50% of the declared ceiling"))
		return false
	}
	return true
}

// client reads the named client's identity, and answers the request when no client holds the name
// or its provider connects through no consent the server holds.
func (s *handlers) client(w http.ResponseWriter, r *http.Request, name string) (clientsetup.SetupClientsRow, bool) {
	stored, err := clientsetup.New(s.opts.Database).SetupClients(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return clientsetup.SetupClientsRow{}, false
	}
	for _, c := range stored {
		if c.ClientName == name {
			if _, ok := s.opts.Consents[c.Provider]; ok {
				return c, true
			}
		}
	}
	server.WriteFailure(w, r, server.ClientFault(http.StatusNotFound, "unknown_client", "no client holds this name"))
	return clientsetup.SetupClientsRow{}, false
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
	s.showAttempt(w, r, func(a attempt) bool { return a.Kind == attemptConnect })
}

// getReauthorize reads the session's attempt to re-authorize or move the path's account.
func (s *handlers) getReauthorize(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	s.showAttempt(w, r, func(a attempt) bool { return a.Kind != attemptConnect && a.Account == account })
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
	s.finish(w, r, func(a attempt) bool { return a.Kind == attemptConnect })
}

// finishReauthorize finishes the session's attempt to re-authorize or move the path's account.
func (s *handlers) finishReauthorize(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	s.finish(w, r, func(a attempt) bool { return a.Kind != attemptConnect && a.Account == account })
}

// errClientChanged is a client replaced or removed since the attempt started.
var errClientChanged = errors.New("the attempt's client was replaced or removed")

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
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "no_attempt", "no connection is in progress in this browser session"))
		return
	}
	if refusal := setup.Expired(a.Expires, s.opts.Clock().UnixMilli()); refusal != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(refusal), "the attempt expired"))
		return
	}
	redirect, err := consent.ReadRedirect(body.Address)
	if err != nil {
		server.RequestLogOf(r.Context()).Err = err
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "wrong_address", "this is not the address the provider redirected to"))
		return
	}
	if refusal := setup.Redirect(redirect, a.State); refusal != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(refusal), "the pasted address cannot finish this attempt"))
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
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "wrong_mailbox", "Google granted access to "+grant.Mailbox+", not "+a.Mailbox+"."))
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
	if a.Kind == attemptConnect {
		listed, err := accounts.New(s.opts.Database).Accounts(r.Context())
		if err != nil {
			server.DatabaseFailure(w, r, err)
			return false
		}
		if slices.ContainsFunc(listed, func(l accounts.AccountsRow) bool { return l.AccountID == a.Account }) {
			server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "identifier_taken", "an account was connected under this identifier while this one was in progress"))
			return false
		}
	}
	stored, err := clientsetup.New(s.opts.Database).SetupClients(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return false
	}
	if !slices.ContainsFunc(stored, func(c clientsetup.SetupClientsRow) bool {
		return c.ClientName == a.Client && c.Provider == a.Provider && c.ClientID == a.ClientID
	}) {
		server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "client_changed", "the client "+a.Client+" changed while this was in progress"))
		return false
	}
	return true
}

// consentFailure answers a code exchange or mailbox read the provider refused or did not answer.
func (s *handlers) consentFailure(w http.ResponseWriter, r *http.Request, a attempt, err error) {
	server.RequestLogOf(r.Context()).Err = err
	switch {
	case errors.Is(err, mail.ErrCodeRefused):
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "code_refused", "the provider refused the code"))
	case errors.Is(err, mail.ErrScopeMissing):
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "scope_missing", "the provider granted no access to the mailbox"))
	case errors.Is(err, mail.ErrScopeRefused):
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "scope_refused", "the provider granted more than the one permission asked for"))
	case errors.Is(err, mail.ErrAPIDisabled):
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "api_disabled", "the provider's API is not enabled in the project of the client "+a.Client))
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
	if len(identity) != 1 || identity[0].Provider != a.Provider || identity[0].ClientID != a.ClientID {
		return errClientChanged
	}
	client := pgtype.Text{String: a.Client, Valid: true}
	mailbox := pgtype.Text{String: a.Mailbox, Valid: true}
	switch a.Kind {
	case attemptConnect:
		if err := q.AddAccount(ctx, accountsetup.AddAccountParams{Provider: a.Provider, OauthClient: client}); err != nil {
			return err
		}
		if err := s.opts.Fault.At("between the account's two rows"); err != nil {
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
		if a.Kind == attemptMove {
			n, err := q.MoveAccount(ctx, accountsetup.MoveAccountParams{OauthClient: client, Provider: a.Provider})
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("the account to move is not listed")
			}
			if err := s.opts.Fault.At("between the client and the credential"); err != nil {
				return err
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
	_, unreachable := server.Violated(err, server.CheckViolation)
	switch {
	case errors.Is(err, errClientChanged):
		server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "client_changed", "the attempt's client changed while this was in progress"))
	case unique && key == accountKey:
		server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "identifier_taken", "an account was connected under this identifier while this one was in progress"))
	case unreachable:
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "identifier_refused", identifierGrammar))
	default:
		server.DatabaseFailure(w, r, err)
	}
	return true
}
