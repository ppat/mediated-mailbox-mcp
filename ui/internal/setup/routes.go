package setup

import (
	"context"
	"crypto/cipher"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/provider/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/setup/clientsecret"
)

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// Database is the UI's connection pool as the setups use it. An account's rows are written and read in
// the transaction helper's transaction set to the account (ADR-0047), and the accounts listing and the
// OAuth clients' identities, which belong to no account, are read on it directly (ADR-0091, ADR-0106).
type Database interface {
	tx.Beginner
	accounts.DBTX
}

// Options are everything the setup routes read. Build builds them from the configuration and the
// process, and the read API endpoint's tests build them by hand, with a fixed clock and the provider
// fake's consents, which no configuration carries.
type Options struct {
	Database Database
	// AttemptCipher is what the consent attempt is sealed with, which the UI's checks derive from
	// their key under the attempt's own label (ADR-0111).
	AttemptCipher cipher.AEAD
	// Clock is the setups' time, which an attempt's expiry and an authentication's time read.
	Clock func() time.Time
	// Seal is the public key the UI seals each client's secret and each credential to. It holds no key
	// that opens one (ADR-0081).
	Seal seal.PublicKey
	// Consents are the consents of the providers whose accounts connect through an OAuth client, by
	// provider, which the setups check clients and connect accounts through (ADR-0106, ADR-0107).
	Consents map[string]mail.Consent[context.Context]
	// Secret is the setups' one operation of the client-secret opener, a client's secret by its name,
	// which a consent's code exchange needs, since Google refuses a desktop client's code exchange
	// without it, PKCE notwithstanding. Nil refuses every code exchange as the UI server failing on its
	// own, so a test that exchanges no code needs none.
	Secret func(ctx context.Context, client string) (string, error)
	// BaseRules counts the base policy's rules, in a base-policy transaction over db, for the
	// installation's read (ADR-0112). It is policy management's, which the read API endpoint hands the
	// setups.
	BaseRules func(ctx context.Context, db tx.Beginner) (int, error)
}

// handlers are the setup routes' handlers over what they read.
type handlers struct {
	opts Options
	keys keys
}

// noSecret refuses a code exchange when the setups were given no way to a client's secret.
func noSecret(context.Context, string) (string, error) {
	return "", errors.New("the setups were given no way to a client's secret")
}

// Build builds the setup routes. It loads the public key the UI seals to and logs its identifier before
// the database is needed, so the key the UI would seal to is in the log whatever the database answers
// (ADR-0092). It then gets the process's pool and the UI's checks, loads the client-secret opener and
// Gmail's consent, and hands the routes the opener's one operation. The key, the opener and the
// consents never leave this package. baseRules is policy management's count of the base rules.
func Build(ctx context.Context, p process.Capabilities, c config.UI, baseRules func(ctx context.Context, db tx.Beginner) (int, error)) ([]server.Route, error) {
	sealKey, err := seal.LoadPublicKey(c.Credential.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading the public key: %w", err)
	}
	p.Logger().Info("sealing to the public key", "key_id", sealKey.KeyID().String())
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	checks, err := server.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	opener, err := clientsecret.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles, pool)
	if err != nil {
		return nil, fmt.Errorf("loading the keyring: %w", err)
	}
	gmailConsent, err := consent.For("gmail", &http.Client{Timeout: providerTimeout}, c.ConsentRedirect)
	if err != nil {
		return nil, err
	}
	return Routes(Options{
		Database: pool, AttemptCipher: checks.AttemptCipher(), Clock: time.Now, Seal: sealKey,
		Consents: map[string]mail.Consent[context.Context]{
			"gmail": gmailConsent,
		},
		Secret: opener.Secret, BaseRules: baseRules,
	}), nil
}

// Routes are the setup routes, each carrying its handler, in the order the read API mounts them: the
// installation's read, the OAuth client setup, connecting an account, and account settings.
func Routes(o Options) []server.Route {
	if o.Secret == nil {
		o.Secret = noSecret
	}
	s := &handlers{opts: o, keys: keys{attempt: o.AttemptCipher}}
	return []server.Route{
		{
			Pattern: paths.Setup, Operation: "getInstallation",
			Summary:  "Each OAuth client's identity and accounts, and every account's identifier, provider and client, reading no account's state",
			Response: installationType(), Handler: s.getInstallation,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/{provider}/clients", Operation: "addClient",
			Summary: "Checks a new OAuth client with its provider and stores it under its name, its secret sealed",
			Request: new(addClientType()), Response: clientAnswerType(), Handler: s.addClient,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/{provider}/clients/{client}", Operation: "replaceClient",
			Summary: "Checks and stores the named client's identifier and secret, or its secret alone",
			Request: new(replaceClientType()), Response: clientAnswerType(), Handler: s.replaceClient,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/{provider}/clients/{client}/remove", Operation: "removeClient",
			Summary:  "Removes a client no account connects through",
			Response: removedType(), Handler: s.removeClient,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/connect", Operation: "startConnect",
			Summary: "Starts the session's consent attempt to connect an account, replacing any it held",
			Request: new(connectType()), Response: attemptAnswerType(), Handler: s.startConnect,
		},
		{
			Pattern: paths.Setup + "/connect", Operation: "getConnect",
			Summary:  "The session's attempt to connect an account, or none",
			Response: attemptAnswerType(), Handler: s.getConnect,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/connect/finish", Operation: "finishConnect",
			Summary: "Finishes the session's attempt from the pasted address and writes the account's two rows",
			Request: new(finishType()), Response: finishAnswerType(), Handler: s.finishConnect,
		},
		{
			Pattern: paths.Accounts + "/{account}/account", Operation: "getAccount", Scoped: true,
			Summary:  "What the UI reads of the account's two rows and its rate state, never the credential",
			Response: accountType(), Handler: s.getAccount,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/account/target", Operation: "setTarget", Scoped: true,
			Summary: "Stores the account's lowered target, or clears it",
			Request: new(targetType()), Response: targetAnswerType(), Handler: s.setTarget,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/account/reauthorize", Operation: "startReauthorize", Scoped: true,
			Summary: "Starts the session's consent attempt to re-authorize the account, or move it to another client",
			Request: new(reauthorizeType()), Response: attemptAnswerType(), Handler: s.startReauthorize,
		},
		{
			Pattern: paths.Accounts + "/{account}/account/reauthorize", Operation: "getReauthorize", Scoped: true,
			Summary:  "The session's attempt to re-authorize the account, or none",
			Response: attemptAnswerType(), Handler: s.getReauthorize,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/account/reauthorize/finish", Operation: "finishReauthorize", Scoped: true,
			Summary: "Finishes the session's attempt from the pasted address and replaces the account's credential",
			Request: new(finishType()), Response: finishAnswerType(), Handler: s.finishReauthorize,
		},
	}
}
