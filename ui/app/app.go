// Package app is the UI's entry package. It exports the UI's assembly, which takes what the UI
// needs as values, starts nothing and returns what it provides, and its composition root, Run,
// which hands the host the UI and its configuration, config.UI (ADR-0040, ADR-0078, ADR-0138).
// ui/main.go calls Run. It sits outside internal, so a composition root other than the UI's own can
// compose it, and every other package of this deployable sits under internal, apart from the
// browser bundle's package in ui/browser, which any root that serves the UI imports.
package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	processdatabase "github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/provider/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/browser"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// Mount is where the UI's handler is served on a process's application server.
var Mount = []string{"/"}

// Run is the UI's process. The host loads the configuration, and the UI is assembled from it over
// the browser bundle ui/browser embeds (ADR-0138).
//
// logger is the logger main.go built over level, at info until the host sets level from log_level
// once the effective configuration is written, so every logger derived from logger follows it
// (ADR-0122). stdout answers --help and --version, and stderr is where the start banner goes,
// standard error in main.go, so standard output carries only the logger's JSON records otherwise.
func Run(ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, host.Spec[config.UI]{
		Name:        "mediated-mailbox-ui",
		Defaults:    config.DefaultUI(runtime.NumCPU()),
		Process:     func(c config.UI) config.Process { return c.Process },
		Operations:  func(c config.UI) config.Operations { return c.Operations },
		Serving:     func(c config.UI) config.Serving { return c.Serving },
		Validate:    config.UI.Check,
		CheckSchema: processdatabase.CheckSchema,
		Components:  []host.Component[config.UI]{{Name: "ui", Mount: Mount, Assemble: fromConfiguration}},
	})
}

// fromConfiguration loads the public key the UI seals to and logs its identifier, then opens the pool
// connecting as the application user, whose schema the host checks as it opens it, so the key the UI would
// seal to is in the log before the start needs the database, and assembles the UI over the embedded
// bundle (ADR-0092, ADR-0140).
func fromConfiguration(ctx context.Context, provided host.Provided[config.UI]) (host.Parts, error) {
	c := provided.Loaded.Config
	bundle, err := browser.Bundle()
	if err != nil {
		return host.Parts{}, err
	}
	sealKey, err := seal.LoadPublicKey(c.Credential.PublicKeyFile)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the public key: %w", err)
	}
	provided.Logger.Info("sealing to the public key", "key_id", sealKey.KeyID().String())
	pool, err := provided.OpenPool(ctx, c.Database, provided.Registerer)
	if err != nil {
		return host.Parts{}, err
	}
	return Assemble(Needs{
		Config: c, SealKey: sealKey, Pool: pool, Bundle: bundle,
		Registerer: provided.Registerer, Logger: provided.Logger, Clock: time.Now, Draining: provided.Draining,
	})
}

// Needs are what the UI is assembled from.
type Needs struct {
	// Config is the UI's configuration. Its credential part names the public key the UI seals to and
	// the private keys only its client-secret package opens with (ADR-0081).
	Config config.UI
	// SealKey is the public key the UI seals to, loaded from the file the credential part names.
	SealKey seal.PublicKey
	// Pool connects as the application user (ADR-0143).
	Pool *pgxpool.Pool
	// Bundle is the browser bundle, rooted at its top level.
	Bundle fs.FS
	// Registerer is where every series of the UI is registered (ADR-0076).
	Registerer prometheus.Registerer
	Logger     *slog.Logger
	// Clock is the UI's time.
	Clock func() time.Time
	// Draining ends when the process begins to stop serving requests, which ends every open event
	// stream.
	Draining context.Context
}

// Assemble builds the UI from n and returns its parts, the UI's handler and its readiness check, the
// database answering as the application user. It loads the request token's key and the client-secret opener,
// and starts nothing.
func Assemble(n Needs) (host.Parts, error) {
	tokenKey, err := loadTokenKey(n.Config.TokenKeyFile)
	if err != nil {
		return host.Parts{}, err
	}
	// The sender classifier's domain functions, which policy management matches senders against
	// rules with, as the deployables that classify pass them (ADR-0004).
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	secrets, err := clientsecret.Load(n.Config.Credential.PublicKeyFile, n.Config.Credential.PrivateKeyFiles, n.Pool)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the keyring: %w", err)
	}
	s := n.Config
	gmailConsent, err := consent.For("gmail", &http.Client{Timeout: providerTimeout}, s.ConsentRedirect)
	if err != nil {
		return host.Parts{}, err
	}
	server, err := api.New(api.Options{
		Bundle:         n.Bundle,
		Database:       n.Pool,
		Datasets:       registry.Datasets(lookups),
		Logger:         n.Logger,
		Metrics:        n.Registerer,
		Draining:       n.Draining,
		Clock:          n.Clock,
		Cadences:       api.Cadences{Sync: time.Duration(s.SyncInterval), Heuristics: time.Duration(s.HeuristicsInterval)},
		StreamInterval: time.Duration(s.StreamInterval),
		Attention: attention.Thresholds{
			BacklogShare: s.AttentionBacklogShare, MaskCount: s.AttentionMaskCount,
			ServeFactor: s.AttentionServeFactor, GapDays: s.AttentionGapDays,
		},
		Browser: api.Browser{
			DefaultTheme: s.DefaultTheme, StreamReconnectMax: time.Duration(s.StreamReconnectMax), StreamPollInterval: time.Duration(s.StreamPollInterval),
			ConsentRedirect: s.ConsentRedirect,
		},
		TokenKey: tokenKey,
		Seal:     n.SealKey,
		Consents: map[string]mail.Consent[context.Context]{
			"gmail": gmailConsent,
		},
		ClientSecrets: secrets,
		Lookups:       lookups,
		Identity:      api.Identity{Header: s.IdentityHeader, Operator: s.OperatorName},
	})
	if err != nil {
		return host.Parts{}, err
	}
	return host.Parts{
		Handler: server.Handler(),
		Ready:   []host.Check{{Name: "database", Check: server.Ready}},
	}, nil
}

// loadTokenKey reads the key behind the request token from its mounted file, or generates one when no
// file is named, in which case a page from before a restart gets stale_page and an attempt from before
// it is lost (ADR-0061, ADR-0111).
func loadTokenKey(path string) ([]byte, error) {
	if path == "" {
		key := make([]byte, api.MinTokenKey)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generating the request token's key: %w", err)
		}
		return key, nil
	}
	key, err := os.ReadFile(path) //nolint:gosec // the path is the mounted key file the configuration names
	if err != nil {
		return nil, fmt.Errorf("reading the request token's key file: %w", err)
	}
	if len(key) < api.MinTokenKey {
		return nil, fmt.Errorf("the request token's key file holds %d bytes, and it needs at least %d", len(key), api.MinTokenKey)
	}
	return key, nil
}
