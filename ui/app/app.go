// Package app is the UI's entry package. It exports the UI's section of the configuration and its
// assembly, which takes what the UI needs as values, starts nothing and returns what it provides, and
// its composition root, Run, which hands the host the UI and the configuration it reads (ADR-0040,
// ADR-0138). ui/main.go calls Run. It sits outside internal, so a composition root other than the UI's
// own can compose it, and every other package of this deployable sits under internal, apart from the
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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/browser"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// Configuration is the UI's root configuration type, loaded by the configuration library under the
// keys docs/UI.md section 18.1 declares (ADR-0078). It composes the process's sections and the UI's
// own section, inline, so every key sits at the top of the tree, with the sections it shares with the
// other deployables. Its fields are pinned by the test beside this file, so a new value is a visible
// change. Only the values the UI reads are declared.
type Configuration struct {
	host.Process    `yaml:",inline"`
	host.Operations `yaml:",inline"`
	host.Serving    `yaml:",inline"`
	Section         `yaml:",inline"`
	Database        dbconnectcore.Config `yaml:"database"`
	// Credential is the credential section the API server and the worker hold, under the same keys. Its
	// public key is the one the UI seals credentials and client secrets to. Its private keys are the
	// keyring the deployables that call a provider open credentials with, which only the UI's
	// client-secret package holds, and it opens a client's secret for a consent's code exchange and
	// nothing else (ADR-0081, ADR-0092).
	Credential credentialcore.Config `yaml:"credential"`
}

// Section is the UI's own section of the configuration, the values only it reads.
type Section struct {
	SyncInterval       time.Duration `yaml:"sync_interval"`
	HeuristicsInterval time.Duration `yaml:"heuristics_interval"`
	StreamInterval     time.Duration `yaml:"stream_interval"`
	DefaultTheme       string        `yaml:"default_theme"`
	StreamReconnectMax time.Duration `yaml:"stream_reconnect_max"`
	StreamPollInterval time.Duration `yaml:"stream_poll_interval"`
	// The worth-a-look thresholds of docs/UI.md section 8.1, each 0 to disable its rule.
	AttentionBacklogShare float64 `yaml:"attention_backlog_share"`
	AttentionMaskCount    int64   `yaml:"attention_mask_count"`
	AttentionServeFactor  float64 `yaml:"attention_serve_factor"`
	AttentionGapDays      int64   `yaml:"attention_gap_days"`
	// TokenKeyFile names a mounted key behind the request token and the consent attempt's seal, which
	// replicas share. Unset, the UI generates one at its assembly (ADR-0061, ADR-0111).
	TokenKeyFile string `yaml:"token_key_file"`
	// ConsentRedirect is the loopback address a consent redirects the browser to, where nothing
	// listens (docs/UI.md sections 8.12 and 18.1).
	ConsentRedirect string `yaml:"consent_redirect"`
	// IdentityHeader names the header an authenticating proxy forwards the operator's identity in, and
	// OperatorName the identity a decision or a policy write records when no header is declared
	// (ADR-0084), so every write has one.
	IdentityHeader string `yaml:"identity_header"`
	OperatorName   string `yaml:"operator_name"`
}

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// DefaultSection is the UI's own section's defaults. The intervals shown on the jobs cards are
// ADR-0018's sync interval and ADR-0117's heuristics run. The server's stream poll and the browser
// stream client's backoff and fallback poll are ADR-0058's, and the browser follows the OS theme. The
// worth-a-look thresholds are section 8.1's starting values. A consent redirects to 127.0.0.1 on a
// high port a web server on the operator's computer is unlikely to answer on, which an installation
// may name otherwise (section 8.12). With no identity header declared, a write records the operator
// name set here, which configuration may replace.
func DefaultSection() Section {
	return Section{
		SyncInterval:          5 * time.Minute,
		HeuristicsInterval:    24 * time.Hour,
		StreamInterval:        2 * time.Second,
		DefaultTheme:          "system",
		StreamReconnectMax:    30 * time.Second,
		StreamPollInterval:    5 * time.Second,
		AttentionBacklogShare: 5,
		AttentionMaskCount:    20,
		AttentionServeFactor:  2,
		AttentionGapDays:      7,
		ConsentRedirect:       "http://127.0.0.1:47823/",
		OperatorName:          "operator",
	}
}

// defaults are the UI's defaults. The user is the UI's own runtime role (ADR-0118), the TLS mode is
// the one that fails closed, and the pool size is the driver's own default.
func defaults() Configuration {
	return Configuration{
		Process:    host.DefaultProcess(),
		Operations: host.DefaultOperations(),
		Serving:    host.DefaultServing(),
		Section:    DefaultSection(),
		Database:   dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_ui", SSLMode: "verify-full", PoolSize: dbconnect.DefaultPoolSize()},
	}
}

// ValidateSection refuses a UI section the UI cannot serve with, through the serving core.
func ValidateSection(s Section) error {
	return serving.Validate(servingConfig(s))
}

// validate refuses a configuration the UI cannot serve with, past the process's sections, which the
// host validates.
func validate(c Configuration) error {
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return err
	}
	if err := ValidateSection(c.Section); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

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
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, host.Spec[Configuration]{
		Name:       "mediated-mailbox-ui",
		Defaults:   defaults(),
		Process:    func(c Configuration) host.Process { return c.Process },
		Operations: func(c Configuration) host.Operations { return c.Operations },
		Serving:    func(c Configuration) host.Serving { return c.Serving },
		Validate:   validate,
		Components: []host.Component[Configuration]{{Name: "ui", Mount: Mount, Assemble: fromConfiguration}},
	})
}

// fromConfiguration opens the pool connecting as the UI's role and assembles the UI over the
// embedded bundle.
func fromConfiguration(ctx context.Context, provided host.Provided[Configuration]) (host.Parts, error) {
	c := provided.Loaded.Config
	bundle, err := browser.Bundle()
	if err != nil {
		return host.Parts{}, err
	}
	pool, err := provided.OpenPool(ctx, c.Database, provided.Registerer)
	if err != nil {
		return host.Parts{}, err
	}
	return Assemble(Needs{
		Section: c.Section, Credential: c.Credential, Pool: pool, Bundle: bundle,
		Registerer: provided.Registerer, Logger: provided.Logger, Clock: time.Now, Draining: provided.Draining,
	})
}

// Needs are what the UI is assembled from.
type Needs struct {
	Section Section
	// Credential names the public key the UI seals to and the private keys only its client-secret
	// package opens with (ADR-0081).
	Credential credentialcore.Config
	// Pool connects as the UI's runtime role (ADR-0118).
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
// database answering as the UI's role. It loads the public key the UI seals to, the request token's
// key and the client-secret opener, and starts nothing.
func Assemble(n Needs) (host.Parts, error) {
	sealKey, err := seal.LoadPublicKey(n.Credential.PublicKeyFile)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the public key: %w", err)
	}
	n.Logger.Info("sealing to the public key", "key_id", sealKey.KeyID().String())
	tokenKey, err := loadTokenKey(n.Section.TokenKeyFile)
	if err != nil {
		return host.Parts{}, err
	}
	// The sender classifier's domain functions, which policy management matches senders against
	// rules with, as the deployables that classify pass them (ADR-0004).
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	secrets, err := clientsecret.Load(n.Credential.PublicKeyFile, n.Credential.PrivateKeyFiles, n.Pool)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the keyring: %w", err)
	}
	s := n.Section
	server, err := api.New(api.Options{
		Bundle:         n.Bundle,
		Database:       n.Pool,
		Datasets:       registry.Datasets(lookups),
		Logger:         n.Logger,
		Metrics:        n.Registerer,
		Draining:       n.Draining,
		Clock:          n.Clock,
		Cadences:       api.Cadences{Sync: s.SyncInterval, Heuristics: s.HeuristicsInterval},
		StreamInterval: s.StreamInterval,
		Attention: attention.Thresholds{
			BacklogShare: s.AttentionBacklogShare, MaskCount: s.AttentionMaskCount,
			ServeFactor: s.AttentionServeFactor, GapDays: s.AttentionGapDays,
		},
		Browser: api.Browser{
			DefaultTheme: s.DefaultTheme, StreamReconnectMax: s.StreamReconnectMax, StreamPollInterval: s.StreamPollInterval,
			ConsentRedirect: s.ConsentRedirect,
		},
		TokenKey: tokenKey,
		Seal:     sealKey,
		Consents: map[string]mail.Consent[context.Context]{
			"gmail": consent.New(&http.Client{Timeout: providerTimeout}, s.ConsentRedirect),
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

// servingConfig is the part of the configuration the serving core validates.
func servingConfig(s Section) serving.Config {
	return serving.Config{
		SyncInterval: int64(s.SyncInterval), HeuristicsInterval: int64(s.HeuristicsInterval), StreamInterval: int64(s.StreamInterval),
		DefaultTheme: s.DefaultTheme, StreamReconnectMax: int64(s.StreamReconnectMax), StreamPollInterval: int64(s.StreamPollInterval),
		AttentionBacklogShare: s.AttentionBacklogShare, AttentionMaskCount: s.AttentionMaskCount,
		AttentionServeFactor: s.AttentionServeFactor, AttentionGapDays: s.AttentionGapDays,
		ConsentRedirect: s.ConsentRedirect,
	}
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
