package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	clientsetup "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/setup"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
)

// maxBody bounds how much of a request body a state-changing route reads.
const maxBody = 64 << 10

// The constraints whose refusals name a client mistake (ADR-0016, ADR-0106).
const (
	clientNameKey     = "oauth_clients_pkey"
	clientIDKey       = "oauth_clients_provider_client_id_key"
	accountKey        = "accounts_pkey"
	accountReachable  = "accounts_account_id_addressable"
	uniqueViolation   = "23505"
	foreignKeyBlocked = "23503"
	checkViolation    = "23514"
)

// violated returns the constraint a database error names, when it is code's violation.
func violated(err error, code string) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == code {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// installationResponse is the installation endpoint, what the installation screens show of the
// stored records, and the count of base rules. It reads no account's state, so nothing on those screens aggregates across accounts
// (docs/UI.md sections 8.10 and 17.4, ADR-0056).
type installationResponse struct {
	Providers []string         `json:"providers"`
	Clients   []clientEntry    `json:"clients"`
	Accounts  []installAccount `json:"accounts"`
	// BaseRules counts the base policy's rules, which belong to no account, read in a base-policy
	// transaction (ADR-0112).
	BaseRules int `json:"base_rules"`
}

type clientEntry struct {
	Name      string   `json:"name"`
	Provider  string   `json:"provider"`
	ClientID  string   `json:"client_id"`
	ProjectID *string  `json:"project_id"`
	Accounts  []string `json:"accounts"`
}

type installAccount struct {
	AccountID   string  `json:"account_id"`
	Provider    string  `json:"provider"`
	OAuthClient *string `json:"oauth_client"`
}

func clientType() schema.Type {
	return schema.Obj("Client",
		schema.F("name", schema.Str()),
		schema.F("provider", schema.Str()),
		schema.F("client_id", schema.Str()),
		schema.F("project_id", schema.Null(schema.Str())),
		schema.F("accounts", schema.ArrayOf(schema.Str())),
	)
}

func installationType() schema.Type {
	return schema.Obj("Installation",
		schema.F("providers", schema.ArrayOf(schema.Str())),
		schema.F("clients", schema.ArrayOf(clientType())),
		schema.F("accounts", schema.ArrayOf(schema.Obj("InstallationAccount",
			schema.F("account_id", schema.Str()),
			schema.F("provider", schema.Str()),
			schema.F("oauth_client", schema.Null(schema.Str())),
		))),
		schema.F("base_rules", schema.Int()),
	)
}

// getInstallation reads every client's identity and every account's listing row. The providers are
// those whose accounts connect through an OAuth client, from the consents the server was given.
func (s *Server) getInstallation(w http.ResponseWriter, r *http.Request) {
	out, err := s.installation(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

func (s *Server) installation(ctx context.Context) (installationResponse, error) {
	clients, err := clientsetup.New(s.opts.Database).SetupClients(ctx)
	if err != nil {
		return installationResponse{}, err
	}
	listed, err := accounts.New(s.opts.Database).Accounts(ctx)
	if err != nil {
		return installationResponse{}, err
	}
	out := installationResponse{Providers: slices.Sorted(maps.Keys(s.opts.Consents)), Clients: []clientEntry{}, Accounts: []installAccount{}}
	for _, c := range clients {
		entry := clientEntry{Name: c.ClientName, Provider: c.Provider, ClientID: c.ClientID, ProjectID: optionalText(c.ProjectID), Accounts: []string{}}
		for _, a := range listed {
			if a.OauthClient.Valid && a.OauthClient.String == c.ClientName {
				entry.Accounts = append(entry.Accounts, a.AccountID)
			}
		}
		out.Clients = append(out.Clients, entry)
	}
	for _, a := range listed {
		out.Accounts = append(out.Accounts, installAccount{AccountID: a.AccountID, Provider: a.AccountProvider, OAuthClient: optionalText(a.OauthClient)})
	}
	if out.BaseRules, err = s.baseRuleCount(ctx); err != nil {
		return installationResponse{}, err
	}
	return out, nil
}

// readBody decodes the request's JSON body into v, admitting no field v does not declare, and answers
// the request when it cannot.
func readBody(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, clientFault(http.StatusBadRequest, "malformed_body", "the request body is not the JSON this request takes"))
		return false
	}
	return true
}

// consentFor returns the consent of the path's provider, and answers the request when no provider of
// that name connects through an OAuth client.
func (s *Server) consentFor(w http.ResponseWriter, r *http.Request, provider string) (mail.Consent[context.Context], bool) {
	c, ok := s.opts.Consents[provider]
	if !ok {
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_provider", "no provider of this name authenticates through an OAuth client"))
	}
	return c, ok
}

type addClientRequest struct {
	Name         string  `json:"name"`
	ClientID     string  `json:"client_id"`
	ClientSecret string  `json:"client_secret"`
	ProjectID    *string `json:"project_id"`
}

type replaceClientRequest struct {
	ClientID     string  `json:"client_id"`
	ClientSecret string  `json:"client_secret"`
	ProjectID    *string `json:"project_id"`
}

type clientAnswer struct {
	Client clientIdentity `json:"client"`
}

type clientIdentity struct {
	Name      string  `json:"name"`
	Provider  string  `json:"provider"`
	ClientID  string  `json:"client_id"`
	ProjectID *string `json:"project_id"`
}

func addClientType() schema.Type {
	return schema.Obj("AddClient",
		schema.F("name", schema.Str()),
		schema.F("client_id", schema.Str()),
		schema.F("client_secret", schema.Str()),
		schema.F("project_id", schema.Null(schema.Str())),
	)
}

func replaceClientType() schema.Type {
	return schema.Obj("ReplaceClient",
		schema.F("client_id", schema.Str()),
		schema.F("client_secret", schema.Str()),
		schema.F("project_id", schema.Null(schema.Str())),
	)
}

func clientAnswerType() schema.Type {
	return schema.Obj("ClientAnswer", schema.F("client", schema.Obj("ClientIdentity",
		schema.F("name", schema.Str()),
		schema.F("provider", schema.Str()),
		schema.F("client_id", schema.Str()),
		schema.F("project_id", schema.Null(schema.Str())),
	)))
}

// addClient checks a new client with the provider and stores it under its name, its secret sealed to
// the row its name keys (docs/UI.md sections 8.11 and 17.4, ADR-0081, ADR-0107). A name the setup
// refuses, a name another client holds and an identifier another client of the provider holds are
// refused before the provider is asked, and again by the table's keys when another setup stored one
// meanwhile. A client the provider refuses or does not answer for is not stored.
func (s *Server) addClient(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	consent, ok := s.consentFor(w, r, provider)
	if !ok {
		return
	}
	var body addClientRequest
	if !readBody(w, r, &body) {
		return
	}
	if refusal := setup.ClientName(body.Name); refusal != setup.None {
		writeFailure(w, r, clientFault(http.StatusBadRequest, string(refusal), "a client's name is 6 to 30 lowercase letters, digits and hyphens starting with a letter, and never new"))
		return
	}
	if !s.validClient(w, r, body.ClientID, body.ClientSecret, body.ProjectID) {
		return
	}
	stored, err := clientsetup.New(s.opts.Database).SetupClients(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	for _, c := range stored {
		if c.ClientName == body.Name {
			writeFailure(w, r, clientFault(http.StatusConflict, "name_taken", "another client holds this name"))
			return
		}
		if c.Provider == provider && c.ClientID == body.ClientID {
			writeFailure(w, r, clientFault(http.StatusConflict, "client_exists", "this client is already set up as "+c.ClientName))
			return
		}
	}
	if !s.checkClient(w, r, consent, body.ClientID, body.ClientSecret) {
		return
	}
	sealed, err := s.opts.Seal.Seal([]byte(body.ClientSecret), seal.ClientSecret(body.Name))
	if err != nil {
		s.uiFailure(w, r, err)
		return
	}
	err = clientsetup.New(s.opts.Database).AddClient(r.Context(), clientsetup.AddClientParams{
		ClientName: body.Name, Provider: provider, ClientID: body.ClientID, ClientSecret: sealed, ProjectID: text(body.ProjectID),
	})
	if key, ok := violated(err, uniqueViolation); ok {
		s.clientConflict(w, r, key)
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, clientAnswer{Client: clientIdentity{Name: body.Name, Provider: provider, ClientID: body.ClientID, ProjectID: body.ProjectID}})
}

// replaceClient replaces the named client's identifier and secret, or with the stored identifier its
// secret alone, which keeps every grant (docs/UI.md section 8.11). It answers as addClient does.
func (s *Server) replaceClient(w http.ResponseWriter, r *http.Request) {
	provider, name := r.PathValue("provider"), r.PathValue("client")
	consent, ok := s.consentFor(w, r, provider)
	if !ok {
		return
	}
	var body replaceClientRequest
	if !readBody(w, r, &body) {
		return
	}
	if !s.validClient(w, r, body.ClientID, body.ClientSecret, body.ProjectID) {
		return
	}
	stored, err := clientsetup.New(s.opts.Database).SetupClients(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	if !slices.ContainsFunc(stored, func(c clientsetup.SetupClientsRow) bool { return c.ClientName == name && c.Provider == provider }) {
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_client", "no client of this provider holds this name"))
		return
	}
	for _, c := range stored {
		if c.ClientName != name && c.Provider == provider && c.ClientID == body.ClientID {
			writeFailure(w, r, clientFault(http.StatusConflict, "client_exists", "this client is already set up as "+c.ClientName))
			return
		}
	}
	if !s.checkClient(w, r, consent, body.ClientID, body.ClientSecret) {
		return
	}
	sealed, err := s.opts.Seal.Seal([]byte(body.ClientSecret), seal.ClientSecret(name))
	if err != nil {
		s.uiFailure(w, r, err)
		return
	}
	n, err := clientsetup.New(s.opts.Database).ReplaceClient(r.Context(), clientsetup.ReplaceClientParams{
		ClientID: body.ClientID, ClientSecret: sealed, ProjectID: text(body.ProjectID), ClientName: name, Provider: provider,
	})
	if key, ok := violated(err, uniqueViolation); ok {
		s.clientConflict(w, r, key)
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	if n == 0 {
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_client", "no client of this provider holds this name"))
		return
	}
	writeJSON(w, r, clientAnswer{Client: clientIdentity{Name: name, Provider: provider, ClientID: body.ClientID, ProjectID: body.ProjectID}})
}

type removedAnswer struct {
	Removed string `json:"removed"`
}

func removedType() schema.Type {
	return schema.Obj("RemovedClient", schema.F("removed", schema.Str()))
}

// removeClient removes a client no account connects through. A client an account connects through is
// refused by the reference the account holds, so it is never removed (ADR-0106).
func (s *Server) removeClient(w http.ResponseWriter, r *http.Request) {
	provider, name := r.PathValue("provider"), r.PathValue("client")
	if _, ok := s.consentFor(w, r, provider); !ok {
		return
	}
	n, err := clientsetup.New(s.opts.Database).RemoveClient(r.Context(), clientsetup.RemoveClientParams{ClientName: name, Provider: provider})
	if _, ok := violated(err, foreignKeyBlocked); ok {
		writeFailure(w, r, clientFault(http.StatusConflict, "client_in_use", "an account connects through this client"))
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	if n == 0 {
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_client", "no client of this provider holds this name"))
		return
	}
	writeJSON(w, r, removedAnswer{Removed: name})
}

// validClient refuses an empty identifier or secret and a project ID not shaped like one.
func (s *Server) validClient(w http.ResponseWriter, r *http.Request, id, secret string, project *string) bool {
	switch {
	case id == "" || secret == "":
		writeFailure(w, r, clientFault(http.StatusBadRequest, "client_refused", "the client identifier and the client secret are both required"))
		return false
	case setup.ProjectID(project) != setup.None:
		writeFailure(w, r, clientFault(http.StatusBadRequest, string(setup.ProjectRefused), "a project ID is 6 to 30 lowercase letters, digits and hyphens starting with a letter"))
		return false
	}
	return true
}

// checkClient asks the provider whether it recognises the client, and answers the request when it
// does not or did not answer. Nothing is stored in either case.
func (s *Server) checkClient(w http.ResponseWriter, r *http.Request, consent mail.Consent[context.Context], id, secret string) bool {
	err := consent.CheckClient(r.Context(), mail.OAuthClient{ID: id, Secret: secret})
	switch {
	case err == nil:
		return true
	case errors.Is(err, mail.ErrClientRefused):
		requestInfo(r.Context()).err = err
		writeFailure(w, r, clientFault(http.StatusBadRequest, "client_refused", "the provider does not recognise this client identifier and secret"))
	default:
		s.providerFailure(w, r, err)
	}
	return false
}

// clientConflict answers a client the table's keys refused, when another setup stored one meanwhile.
func (s *Server) clientConflict(w http.ResponseWriter, r *http.Request, key string) {
	switch key {
	case clientNameKey:
		writeFailure(w, r, clientFault(http.StatusConflict, "name_taken", "another client holds this name"))
	case clientIDKey:
		writeFailure(w, r, clientFault(http.StatusConflict, "client_exists", "this client is already set up under another name"))
	default:
		writeFailure(w, r, databaseFault())
	}
}

// providerFailure reports a provider that did not answer a setup request. A cancelled request is the
// client going away.
func (s *Server) providerFailure(w http.ResponseWriter, r *http.Request, err error) {
	requestInfo(r.Context()).err = err
	if errors.Is(err, context.Canceled) {
		return
	}
	writeFailure(w, r, providerFault())
}

// uiFailure reports the UI server failing on its own.
func (s *Server) uiFailure(w http.ResponseWriter, r *http.Request, err error) {
	requestInfo(r.Context()).err = err
	writeFailure(w, r, uiFault())
}

// text is an optional string as the generated statements take it.
func text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
