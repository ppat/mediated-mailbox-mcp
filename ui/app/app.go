// Package app is the UI's entry package and composition root. It constructs the object graph by hand
// in ordinary code, and ui/main.go calls it. It sits outside internal, so a composition root other
// than the UI's own can compose it, and every other package of this deployable sits under internal.
//
// The browser bundle reaches it from ui/main.go, which embeds it, and it passes the bundle into the
// server, so no other package reaches for it.
package app

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/devloop"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// Configuration is the UI's root configuration type, loaded by the configuration library under the
// keys docs/UI.md section 18.1 declares (ADR-0078). Its fields are pinned by the test beside this
// file, so a new value is a visible change. Only the values the UI reads are declared.
type Configuration struct {
	Database           dbconnectcore.Config `yaml:"database"`
	Listen             string               `yaml:"listen"`
	ProbeListen        string               `yaml:"probe_listen"`
	TLSCert            string               `yaml:"tls_cert"`
	TLSKey             string               `yaml:"tls_key"`
	InsecureHTTP       bool                 `yaml:"insecure_http"`
	SyncInterval       time.Duration        `yaml:"sync_interval"`
	HeuristicsInterval time.Duration        `yaml:"heuristics_interval"`
	StreamInterval     time.Duration        `yaml:"stream_interval"`
	DefaultTheme       string               `yaml:"default_theme"`
	StreamReconnectMax time.Duration        `yaml:"stream_reconnect_max"`
	StreamPollInterval time.Duration        `yaml:"stream_poll_interval"`
	// The worth-a-look thresholds of docs/UI.md section 8.1, each 0 to disable its rule.
	AttentionBacklogShare float64 `yaml:"attention_backlog_share"`
	AttentionMaskCount    int64   `yaml:"attention_mask_count"`
	AttentionServeFactor  float64 `yaml:"attention_serve_factor"`
	AttentionGapDays      int64   `yaml:"attention_gap_days"`
	// SealPublicKeyFile names the mounted public key the UI seals credentials and client secrets to.
	SealPublicKeyFile string `yaml:"seal_public_key_file" settings:"required"`
	// PrivateKeyFiles names every mounted private key, the keyring the deployables that call a provider
	// open credentials with. Only the UI's client-secret package holds it, and it opens a client's
	// secret for a consent's code exchange and nothing else (ADR-0081, ADR-0092).
	PrivateKeyFiles []string `yaml:"private_key_files" settings:"required"`
	// TokenKeyFile names a mounted key behind the request token and the consent attempt's seal, which
	// replicas share. Unset, the UI generates one at its start (ADR-0061, ADR-0111).
	TokenKeyFile string `yaml:"token_key_file"`
	// ConsentRedirect is the loopback address a consent redirects the browser to, where nothing
	// listens (docs/UI.md sections 8.12 and 18.1).
	ConsentRedirect string `yaml:"consent_redirect"`
	// IdentityHeader names the header an authenticating proxy forwards the operator's identity in, and
	// OperatorName the identity a decision or a policy write records when no header is declared
	// (ADR-0084), so every write has one.
	IdentityHeader string `yaml:"identity_header"`
	OperatorName   string `yaml:"operator_name"`
	// LogLevel is the lowest level the deployable logs at, debug, info, warn or error (ADR-0122).
	LogLevel string `yaml:"log_level"`
}

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// defaults are the UI's defaults. The user is the UI's own runtime role (ADR-0075), the TLS mode is
// the one that fails closed, the two listen addresses are the mediator's, and the intervals shown
// on the jobs cards are ADR-0018's sync interval and ADR-0022's heuristics run. The server's stream
// poll and the browser stream client's backoff and fallback poll are ADR-0058's, and the browser
// follows the OS theme. The worth-a-look thresholds are section 8.1's starting values. A consent
// redirects to 127.0.0.1 on a high port a web server on the operator's computer is unlikely to
// answer on, which an installation may name otherwise (section 8.12). With no identity header
// declared, a write records the operator name set here, which configuration may replace.
func defaults() Configuration {
	return Configuration{
		Database:              dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_ui", SSLMode: "verify-full"},
		Listen:                ":8443",
		ProbeListen:           ":8080",
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
		LogLevel:              logging.DefaultLevel,
	}
}

// Run reads the configuration, validates it, builds the server over the browser bundle, embedded
// holding it under browser/dist, and serves the UI and the probes until ctx ends. It logs the
// effective configuration first, each value with the layer that set it.
//
// logger is the logger main.go built over level, at info until Run sets level from log_level once
// the effective configuration is written, so every logger derived from logger follows it (ADR-0122).
func Run(ctx context.Context, args, environ []string, logger *slog.Logger, level *slog.LevelVar, embedded fs.FS) error {
	if err := dbconnect.RefusePasswordVariables(environ); err != nil {
		return err
	}
	loaded, err := settings.Load(defaults(), args, environ)
	var help *settings.HelpRequested
	if errors.As(err, &help) {
		fmt.Print(help.Text)
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading the configuration: %w", err)
	}
	// The effective configuration is written before the level applies, so a refused value's source,
	// log_level's included, reaches the log whatever level the configuration sets (ADR-0078, ADR-0122).
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	lv, err := logging.ParseLevel(loaded.Config.LogLevel)
	if err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	level.Set(lv)
	c := loaded.Config
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	if err := serving.Validate(serving.Config{
		Listen: c.Listen, ProbeListen: c.ProbeListen, TLSCert: c.TLSCert, TLSKey: c.TLSKey, InsecureHTTP: c.InsecureHTTP,
		SyncInterval: int64(c.SyncInterval), HeuristicsInterval: int64(c.HeuristicsInterval), StreamInterval: int64(c.StreamInterval),
		DefaultTheme: c.DefaultTheme, StreamReconnectMax: int64(c.StreamReconnectMax), StreamPollInterval: int64(c.StreamPollInterval),
		AttentionBacklogShare: c.AttentionBacklogShare, AttentionMaskCount: c.AttentionMaskCount,
		AttentionServeFactor: c.AttentionServeFactor, AttentionGapDays: c.AttentionGapDays,
		ConsentRedirect: c.ConsentRedirect,
	}, devloop.Enabled()); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	bundle, err := fs.Sub(embedded, "browser/dist")
	if err != nil {
		return err
	}
	sealKey, err := seal.LoadPublicKey(c.SealPublicKeyFile)
	if err != nil {
		return fmt.Errorf("loading the public key: %w", err)
	}
	logger.Info("sealing to the public key", "key_id", sealKey.KeyID().String())
	tokenKey, err := loadTokenKey(c.TokenKeyFile)
	if err != nil {
		return err
	}
	poolConfig, err := dbconnect.PoolConfig(c.Database)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	defer pool.Close()
	// The sender classifier's domain functions, which policy management matches senders against
	// rules with, as the deployables that classify pass them (ADR-0004).
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	secrets, err := clientsecret.Load(c.SealPublicKeyFile, c.PrivateKeyFiles, pool)
	if err != nil {
		return fmt.Errorf("loading the keyring: %w", err)
	}
	server, err := api.New(api.Options{
		Bundle:         bundle,
		Database:       pool,
		Datasets:       registry.Datasets(lookups),
		Logger:         logger,
		Metrics:        prometheus.NewRegistry(),
		Clock:          time.Now,
		Cadences:       api.Cadences{Sync: c.SyncInterval, Heuristics: c.HeuristicsInterval},
		StreamInterval: c.StreamInterval,
		Attention: attention.Thresholds{
			BacklogShare: c.AttentionBacklogShare, MaskCount: c.AttentionMaskCount,
			ServeFactor: c.AttentionServeFactor, GapDays: c.AttentionGapDays,
		},
		Browser: api.Browser{
			DefaultTheme: c.DefaultTheme, StreamReconnectMax: c.StreamReconnectMax, StreamPollInterval: c.StreamPollInterval,
			ConsentRedirect: c.ConsentRedirect,
		},
		TokenKey: tokenKey,
		Seal:     sealKey,
		Consents: map[string]mail.Consent[context.Context]{
			"gmail": consent.New(&http.Client{Timeout: providerTimeout}, c.ConsentRedirect),
		},
		ClientSecrets: secrets,
		Lookups:       lookups,
		Identity:      api.Identity{Header: c.IdentityHeader, Operator: c.OperatorName},
	})
	if err != nil {
		return err
	}
	return serve(ctx, c, server, logger)
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

// serve runs the UI's listener, TLS unless the dev loop's plain HTTP is configured, and the probes'
// plain-HTTP listener, until ctx ends or either fails, then shuts both down.
func serve(ctx context.Context, c Configuration, server *api.Server, logger *slog.Logger) error {
	if !c.InsecureHTTP {
		if _, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey); err != nil {
			return fmt.Errorf("loading the TLS key pair: %w", err)
		}
	}
	var lc net.ListenConfig
	uiListener, err := lc.Listen(ctx, "tcp", c.Listen)
	if err != nil {
		return err
	}
	probeListener, err := lc.Listen(ctx, "tcp", c.ProbeListen)
	if err != nil {
		return errors.Join(err, uiListener.Close())
	}
	// Requests take ctx as their base, so an open event stream ends when the process is told to stop
	// rather than holding the shutdown until its timeout.
	ui := &http.Server{
		Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second, TLSConfig: tlsConfig(c.TLSCert, c.TLSKey),
		BaseContext: func(net.Listener) context.Context { return ctx },
		ErrorLog:    logging.ServerErrorLog(logger),
	}
	probes := &http.Server{Handler: server.Probes(), ReadHeaderTimeout: 10 * time.Second, ErrorLog: logging.ServerErrorLog(logger)}
	errs := make(chan error, 2)
	go func() {
		if c.InsecureHTTP {
			errs <- ui.Serve(uiListener)
			return
		}
		errs <- ui.ServeTLS(uiListener, "", "")
	}()
	go func() { errs <- probes.Serve(probeListener) }()
	logger.Info("serving", "listen", c.Listen, "probe_listen", c.ProbeListen, "tls", !c.InsecureHTTP)
	select {
	case <-ctx.Done():
	case err = <-errs:
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, ui.Shutdown(shutdown), probes.Shutdown(shutdown))
}

// tlsConfig reads the key pair from its mounted files on each handshake, so a renewed certificate is
// served without a restart, as the mediator's client surface does (docs/UI.md section 15).
func tlsConfig(certFile, keyFile string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			pair, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("loading the TLS key pair: %w", err)
			}
			return &pair, nil
		},
	}
}
