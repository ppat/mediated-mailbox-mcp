// Command ui is the UI, published as mediated-mailbox-ui.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
//
// The browser bundle is embedded here and passed into the server, so no other package reaches for it.
// The pattern carries the all: prefix because a fresh clone holds only the checked-in placeholder in
// browser/dist, and without the prefix a directory holding only a dot file fails to compile. The
// image build refuses a bundle that is only the placeholder (ui/Dockerfile).
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

//go:embed all:browser/dist
var embedded embed.FS

// Configuration is the UI's root configuration type, loaded by the configuration library under the
// keys docs/UI.md section 18.1 declares (ADR-0078). Its fields are pinned by the test beside this
// file, so a new value is a visible change. Only the values the UI reads are declared.
type Configuration struct {
	Database           dbconnectcore.Config `yaml:"database"`
	Listen             string               `yaml:"listen"`
	ProbeListen        string               `yaml:"probe_listen"`
	TLSCert            string               `yaml:"tls_cert"`
	TLSKey             string               `yaml:"tls_key"`
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
}

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// defaults are the UI's defaults. The user is the UI's own runtime role (ADR-0075). No TLS file is
// named, so the UI serves plain HTTP until a certificate and a key are (ADR-0118). The two listen
// addresses are the mediator's, the intervals shown on the jobs cards are ADR-0018's sync interval
// and ADR-0022's daily heuristics run, and the stream polls every two seconds. The browser follows the OS theme, and its stream client backs off to 30 seconds
// and polls every 5 seconds after its fallback (ADR-0058). The worth-a-look thresholds are section 8.1's
// starting values. A consent redirects to 127.0.0.1 on a high port a web server on the operator's
// computer is unlikely to answer on, which an installation may name otherwise (section 8.12). With no
// identity header declared, a write records the operator name, operator unless configured otherwise.
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
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	err := run(ctx, os.Args[1:], os.Environ(), logger)
	stop()
	if err != nil {
		logger.Error("ui stopped", "error", err)
		os.Exit(1)
	}
}

// run reads the configuration, validates it, builds the server and serves the UI and the probes until
// ctx ends. It logs the effective configuration first, each value with the layer that set it.
func run(ctx context.Context, args, environ []string, logger *slog.Logger) error {
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
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	c := loaded.Config
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	if err := serving.Validate(serving.Config{
		Listen: c.Listen, ProbeListen: c.ProbeListen, TLSCert: c.TLSCert, TLSKey: c.TLSKey,
		SyncInterval: int64(c.SyncInterval), HeuristicsInterval: int64(c.HeuristicsInterval), StreamInterval: int64(c.StreamInterval),
		DefaultTheme: c.DefaultTheme, StreamReconnectMax: int64(c.StreamReconnectMax), StreamPollInterval: int64(c.StreamPollInterval),
		AttentionBacklogShare: c.AttentionBacklogShare, AttentionMaskCount: c.AttentionMaskCount,
		AttentionServeFactor: c.AttentionServeFactor, AttentionGapDays: c.AttentionGapDays,
		ConsentRedirect: c.ConsentRedirect,
	}); err != nil {
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
	opts := configured(c)
	opts.Bundle, opts.Database, opts.Datasets, opts.Logger = bundle, pool, registry.Datasets(lookups), logger
	opts.Metrics, opts.Clock, opts.TokenKey, opts.Seal = prometheus.NewRegistry(), time.Now, tokenKey, sealKey
	opts.Consents = map[string]mail.Consent[context.Context]{
		"gmail": consent.New(&http.Client{Timeout: providerTimeout}, c.ConsentRedirect),
	}
	opts.ClientSecrets, opts.Lookups = secrets, lookups
	server, err := api.New(opts)
	if err != nil {
		return err
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
	return serve(ctx, c, uiListener, probeListener, server, logger)
}

// configured returns the server's options its configuration decides, which run completes with the
// server's dependencies. The mode the UI serves in sets its cookies' Secure attribute, from the same
// source as the listener's mode (ADR-0118).
func configured(c Configuration) api.Options {
	return api.Options{
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
		Identity:  api.Identity{Header: c.IdentityHeader, Operator: c.OperatorName},
		ServesTLS: c.servesTLS(),
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

// serve serves the UI on uiListener, over TLS when a certificate and a key are named and over plain
// HTTP when neither is (ADR-0118), and the probes on probeListener over plain HTTP, until ctx ends or
// either fails, then shuts both down. It takes the listeners already open, as the mediator's serve
// does, so a caller that must know the addresses holds them from before the start, and it closes both
// on every path. A key pair that is named and does not load stops it before it serves.
func serve(ctx context.Context, c Configuration, uiListener, probeListener net.Listener, server *api.Server, logger *slog.Logger) error {
	overTLS := c.servesTLS()
	if overTLS {
		if _, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey); err != nil {
			return errors.Join(fmt.Errorf("loading the TLS key pair: %w", err), uiListener.Close(), probeListener.Close())
		}
	}
	// Requests take ctx as their base, so an open event stream ends when the process is told to stop
	// rather than holding the shutdown until its timeout.
	ui := &http.Server{
		Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second, TLSConfig: tlsConfig(c.TLSCert, c.TLSKey),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	probes := &http.Server{Handler: server.Probes(), ReadHeaderTimeout: 10 * time.Second}
	errs := make(chan error, 2)
	go func() { errs <- serveUI(ui, uiListener, overTLS) }()
	go func() { errs <- probes.Serve(probeListener) }()
	logger.Info("serving", "listen", c.Listen, "probe_listen", c.ProbeListen, "tls", overTLS)
	var err error
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

// servesTLS is whether the UI serves TLS, which both TLS files named decide. It is the one source of
// the mode, which the listener and the cookies' Secure attribute both follow (ADR-0118).
func (c Configuration) servesTLS() bool { return c.TLSCert != "" && c.TLSKey != "" }

// serveUI serves the UI on ln until it is shut down, over TLS with srv's TLS configuration when
// overTLS is set and over plain HTTP otherwise. The caller derives overTLS from the configuration
// naming a key pair (ADR-0118), so a server built without its TLS configuration fails its start
// rather than serving plain HTTP.
func serveUI(srv *http.Server, ln net.Listener, overTLS bool) error {
	if !overTLS {
		return srv.Serve(ln)
	}
	return srv.ServeTLS(ln, "", "")
}

// tlsConfig reads the key pair from its mounted files on each handshake, so a renewed certificate is
// served without a restart, as the mediator's client surface does (docs/UI.md section 15). It is used
// only when the UI is served over TLS.
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
