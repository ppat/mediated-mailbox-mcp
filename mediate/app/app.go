// Package app is the mediator's entry package and composition root. It constructs the object graph by
// hand in ordinary code, and mediate/main.go calls it. It sits outside internal, so a composition root
// other than the mediator's own can compose it. Every other package of this deployable sits under
// internal, apart from importtarget, which exists only under the banproof tag.
//
// The mediator listens twice. The client surface, the API root under /api/ and the MCP root at
// /mcp, is served over TLS unless the configuration declares that an ingress in front terminates
// TLS, and every request to it passes the bearer check before it reaches either root (ADR-0030,
// ADR-0087). The mediator is ready once its accounts and their policy have loaded, both listeners
// are up, and the TLS key pair and the bearer token have loaded. The probes and the metrics endpoint
// are served over plain HTTP on a listener of their own, because a platform's healthcheck and
// scraper carry no bearer token (ADR-0051). They are operational endpoints outside the operation
// registry, and the client surface's listener never serves them.
//
// It reads its configuration through the configuration library (ADR-0078), and its database
// connection is built from the configuration's database section, with the password in the mounted
// file the section names. It refuses to start while PGPASSWORD or PGSSLPASSWORD is set, so no
// password arrives by the environment (ADR-0078, ADR-0079). The accounts it serves live in the
// database and never in its configuration (ADR-0080). They reach the registry, the policy loader and
// the rate-state collector as an account snapshot, loaded at start and reloaded on the interval the
// configuration sets (ADR-0090). The registry refuses every call naming an account the snapshot in
// force does not list. The TLS key pair and the bearer token are mounted files, read again on each
// handshake and each request, so a rotated file takes effect without a restart (ADR-0079, ADR-0080).
// The mediator also refuses to start while MCPGODEBUG is set, because the MCP SDK reads it to switch
// its behaviour (ADR-0086).
package app

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/reload"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// gmailProvider is the provider an account served through the Gmail adapter names in accounts.
const gmailProvider = "gmail"

// Configuration is the mediator's root configuration type, loaded by the configuration library
// (ADR-0078). Its fields are pinned by the test beside this file, so a new value is a visible
// change. No account and no provider credential is configuration. Both come from the database
// (ADR-0080).
type Configuration struct {
	// Listen is the address the client surface listens on.
	Listen string `yaml:"listen"`
	// ProbeListen is the address the probes and the metrics endpoint listen on, over plain HTTP.
	ProbeListen string `yaml:"probe_listen"`
	// TLSAtIngress declares that an ingress in front of the mediator terminates TLS, so the client
	// surface listens over plain HTTP and takes no key pair.
	TLSAtIngress bool `yaml:"tls_at_ingress"`
	// TLSCert and TLSKey name the mounted files holding the client surface's certificate chain and
	// private key.
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
	// TokenFile names the mounted file holding the bearer token clients present.
	TokenFile string `yaml:"token_file" settings:"required"`
	// AccountReloadInterval is how often the account snapshot is reloaded (ADR-0090).
	AccountReloadInterval time.Duration         `yaml:"account_reload_interval"`
	Database              dbconnectcore.Config  `yaml:"database"`
	Credential            credentialcore.Config `yaml:"credential"`
	// Scanner is the scanner's section, the same section backfill reads, whose pattern tier the
	// serve-time pattern check runs (ADR-0002, ADR-0005). No value in it switches the check off.
	Scanner scan.Config `yaml:"scanner"`
	// ProviderTimeout bounds each provider call a body request makes, after which the call is the
	// provider's failure (ADR-0101).
	ProviderTimeout time.Duration `yaml:"provider_timeout"`
	// LogLevel is the lowest level the deployable logs at, debug, info, warn or error (ADR-0122).
	LogLevel string `yaml:"log_level"`
}

// maxProviderTimeout is the longest provider timeout the configuration accepts.
const maxProviderTimeout = 5 * time.Minute

// scannerSection is the scanner's section of the configuration, whose revision the serve-time
// pattern check runs under (ADR-0078).
const scannerSection = "scanner"

// defaults are the mediator's defaults. The user is the mediator's own runtime role (ADR-0118), and
// the TLS mode is the one that fails closed. The TLS, token and key files have no default, since a
// default path assumes the environment.
func defaults() Configuration {
	return Configuration{
		Listen:                ":8443",
		ProbeListen:           ":8080",
		AccountReloadInterval: time.Minute,
		Database:              dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_mediate", SSLMode: "verify-full"},
		Scanner:               scan.DefaultConfig(),
		ProviderTimeout:       30 * time.Second,
		LogLevel:              logging.DefaultLevel,
	}
}

// validate refuses a configuration the mediator cannot serve with. The TLS files are required unless
// an ingress is declared to terminate TLS, and refused when one is. The reload interval is positive,
// and the provider timeout positive and at most five minutes.
func validate(c Configuration) error {
	if c.TLSAtIngress {
		if c.TLSCert != "" || c.TLSKey != "" {
			return errors.New("tls_at_ingress declares that the mediator serves no TLS, so it takes no tls_cert or tls_key")
		}
	} else {
		var missing []string
		if c.TLSCert == "" {
			missing = append(missing, "tls_cert")
		}
		if c.TLSKey == "" {
			missing = append(missing, "tls_key")
		}
		if len(missing) > 0 {
			return fmt.Errorf("the mediator serves TLS unless tls_at_ingress is set, so it needs %s", strings.Join(missing, " and "))
		}
	}
	if strings.TrimSpace(c.TokenFile) == "" {
		return errors.New("token_file is empty")
	}
	if c.AccountReloadInterval <= 0 {
		return fmt.Errorf("account_reload_interval %s is not positive", c.AccountReloadInterval)
	}
	if c.ProviderTimeout <= 0 || c.ProviderTimeout > maxProviderTimeout {
		return fmt.Errorf("provider_timeout %s is not positive and at most %s", c.ProviderTimeout, maxProviderTimeout)
	}
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

// Run loads the configuration and the keyring, connects to the database, loads the accounts and
// their policy, and serves until ctx ends. It logs the effective configuration first, each value
// with the layer that set it (ADR-0078). The keyring is loaded before any connection is made, so a
// public key matching none of the private keys refuses the start (ADR-0088).
//
// logger is the logger main.go built over level, at info until Run sets level from log_level once
// the effective configuration is written, so every logger derived from logger follows it (ADR-0122).
func Run(ctx context.Context, args, environ []string, logger *slog.Logger, level *slog.LevelVar) error {
	if err := refuseEnvironment(environ); err != nil {
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
	if err := validate(c); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	scanner, err := scan.New(c.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return fmt.Errorf("validating the configuration: scanner: %w", err)
	}
	keys, err := open.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles...)
	if err != nil {
		return fmt.Errorf("loading the keyring: %w", err)
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
	metrics := prometheus.NewRegistry()
	served, err := newServing(ctx, pool, keys, metrics, logger)
	if err != nil {
		return err
	}
	connect, err := gmailConnector(metrics)
	if err != nil {
		return err
	}
	bodies, _, err := newBodies(served, connect, scanner, c.ProviderTimeout, metrics)
	if err != nil {
		return err
	}
	registry, err := service.NewRegistry(nil, service.Operations(sources(pool, served, bodies))...)
	if err != nil {
		return err
	}
	served.registry = registry
	if err := served.reload(ctx); err != nil {
		return err
	}
	surfaceListener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	probeListener, err := net.Listen("tcp", c.ProbeListen)
	if err != nil {
		return errors.Join(err, surfaceListener.Close())
	}
	return serve(ctx, c, surfaceListener, probeListener, served, metrics, logger)
}

// serve reloads the account snapshot on its interval and serves the registry on the two listeners
// until ctx ends. It takes the listeners already open, so a caller that must know the addresses
// holds them from before the start, and it closes both on every path.
func serve(ctx context.Context, c Configuration, surfaceListener, probeListener net.Listener, served *serving, metrics *prometheus.Registry, logger *slog.Logger) error {
	registry := served.registry
	var ready readiness.State

	surfaceHandler, err := surface(registry, c.TokenFile, logger)
	if err != nil {
		return errors.Join(err, surfaceListener.Close(), probeListener.Close())
	}
	surfaceServer := &http.Server{
		Handler:           surfaceHandler,
		TLSConfig:         tlsConfig(c.TLSCert, c.TLSKey, c.TLSAtIngress),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          logging.ServerErrorLog(logger),
	}
	probeServer := &http.Server{
		Handler:           probes(&ready, metrics, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          logging.ServerErrorLog(logger),
	}
	reloading, stopReloading := context.WithCancel(ctx)
	defer stopReloading()
	// One slot for each server and one for the readiness check, so no send blocks.
	errs := make(chan error, 3)
	go func() { errs <- serveSurface(surfaceServer, surfaceListener) }()
	go func() { errs <- serveProbes(probeServer, probeListener) }()
	go served.reloadEvery(reloading, c.AccountReloadInterval)
	if err := markReadyWhenServable(&ready, c); err != nil {
		errs <- err
	}
	logger.Info("serving", "surface", c.Listen, "probes", c.ProbeListen)

	select {
	case err = <-errs:
	case <-ctx.Done():
	}
	ready.MarkNotReady()
	stopReloading()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(err, surfaceServer.Shutdown(shutdown), probeServer.Shutdown(shutdown))
}

// sources are what the operations read from, the pool, the accounts and policy the serving state
// holds in force, the classifier's lookups, the clock, and what the body operation needs beyond them.
func sources(pool *pgxpool.Pool, served *serving, bodies service.Bodies) service.Sources {
	return service.Sources{
		DB:       pool,
		Accounts: served.accounts,
		Policy:   served.policy,
		Lookups:  lookups(),
		Now:      time.Now,
		Bodies:   bodies,
	}
}

// lookups are the classifier's domain functions, the UTS #46 lookup profile and the public-suffix
// list, which the pure classifier takes as parameters.
func lookups() classify.Lookups {
	return classify.Lookups{
		ToUnicode:   idna.Lookup.ToUnicode,
		ToASCII:     idna.Lookup.ToASCII,
		Registrable: publicsuffix.EffectiveTLDPlusOne,
	}
}

// serving is the set of accounts the mediator serves and their policy. Each reload takes a new
// account snapshot, serves its accounts and loads their policy, and an account whose rules no load
// has read restricts every sender (ADR-0090, ADR-0041). A body request has the policy loaded again
// before it decides, through the same coalesced load the reloads use (ADR-0099).
type serving struct {
	loader  *accountload.Loader
	pool    *pgxpool.Pool
	metrics *prometheus.Registry
	logger  *slog.Logger
	// policies is the policy loader, built at the first reload that serves an account. Calls read it
	// while a reload may be building it.
	policies atomic.Pointer[policyload.Loader]
	// fresh runs the policy loads, each started after every caller it answers asked (ADR-0099).
	fresh    *reload.Coalesced
	registry service.Registry
	// snapshot is the account snapshot the registry serves.
	snapshot atomic.Pointer[accountload.Snapshot]
}

// newServing returns the serving state, serving no account until its first reload, and registers on
// metrics F3's collector of each served account's rate-state series, which the mediator carries
// because it runs continuously (ADR-0077). Its policy loads run under ctx, so they end when the
// mediator stops and never because a client stopped waiting.
func newServing(ctx context.Context, pool *pgxpool.Pool, keys *open.Keyring, metrics *prometheus.Registry, logger *slog.Logger) (*serving, error) {
	s := &serving{loader: accountload.New(pool, keys, logger, []string{gmailProvider}), pool: pool, metrics: metrics, logger: logger}
	s.fresh = reload.New(ctx, s.reloadPolicy)
	s.snapshot.Store(&accountload.Snapshot{})
	if err := metrics.Register(lease.NewCollector(pool, s.rateAccounts)); err != nil {
		return nil, err
	}
	return s, nil
}

// reload loads the account snapshot and the policy of its accounts, and serves them. A snapshot read
// that fails keeps the accounts served and their policy, and returns the error, which the start
// treats as fatal and a scheduled reload logs. A read listing no account serves none, and an account a
// read no longer lists is dropped (ADR-0090).
//
// The policy is loaded on every reload, so a policy edit reaches the metadata reads and the
// reload-failure alarm clears without a body request (ADR-0099). A policy load that fails is
// returned, and the snapshot's accounts are served all the same, so an account the snapshot drops is
// dropped. The policy loader keeps its active snapshot, so an account it read before keeps that
// policy, and an account it never read gets the policy that restricts every sender (ADR-0041). The
// next reload loads the policy again.
func (s *serving) reload(ctx context.Context) error {
	if err := s.loader.Load(ctx); err != nil {
		return fmt.Errorf("loading the accounts: %w", err)
	}
	next := s.loader.Snapshot()
	ids := idsOf(next)
	failed := s.loadPolicy(ctx, ids)
	s.snapshot.Store(next)
	s.registry.Serve(ids)
	s.logger.Info("accounts served", "accounts", len(ids))
	return failed
}

// loadPolicy sets the accounts the shared policy loader reads to accounts and has it load, through
// the coalesced load. With no account there is no policy to load.
func (s *serving) loadPolicy(ctx context.Context, accounts []string) error {
	if len(accounts) == 0 {
		s.logger.Info("no account is served, so no policy is loaded")
		return nil
	}
	policies := s.policies.Load()
	if policies == nil {
		built, err := policyload.New(s.pool, accounts, s.metrics)
		if err != nil {
			return err
		}
		s.policies.Store(built)
	} else if err := policies.SetAccounts(accounts); err != nil {
		return err
	}
	if err := s.fresh.Load(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	s.logger.Info("policy loaded", "accounts", len(accounts))
	return nil
}

// reloadPolicy is one policy load, of the accounts the policy loader was last set to read. Before the
// first reload that serves an account there is no loader and nothing to load.
func (s *serving) reloadPolicy(ctx context.Context) error {
	policies := s.policies.Load()
	if policies == nil {
		return nil
	}
	return policies.Reload(ctx)
}

// currentPolicy has the policy loaded by a load that starts after the call, and returns the account's
// policy from the snapshot then active (ADR-0099). A load that fails leaves the active valid policy,
// whose failure the loader alarms on (ADR-0041). A caller that stops waiting gets the policy that
// restricts every sender, so a decision is never made against a policy the load did not reach.
func (s *serving) currentPolicy(ctx context.Context, account string) policy.Composed {
	if err := s.fresh.Load(ctx); err != nil {
		if ctx.Err() != nil {
			return policy.Composed{}
		}
		s.logger.Error("the policy was not loaded, so the active policy decides", "error", err)
	}
	return s.policy(account)
}

// reloadEvery reloads on every tick of interval until ctx ends. A failed reload is logged and keeps
// what was served.
func (s *serving) reloadEvery(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reload(ctx); err != nil && ctx.Err() == nil {
				s.logger.Error("the accounts were not reloaded, so the ones served stay", "error", err)
			}
		}
	}
}

// accounts returns the accounts served with their providers, for the accounts listing.
func (s *serving) accounts() []service.Account {
	var out []service.Account
	for _, a := range s.snapshot.Load().Accounts() {
		out = append(out, service.Account{ID: a.ID(), Provider: a.Provider()})
	}
	return out
}

// policy returns the policy an account's messages are decided against. With no policy loaded it is
// the policy that restricts every sender.
func (s *serving) policy(account string) policy.Composed {
	policies := s.policies.Load()
	if policies == nil {
		return policy.Composed{}
	}
	return policies.Snapshot().For(account)
}

// rateAccounts returns the accounts the rate-state collector reports on, every served account whose
// provider the mediator has a rate profile for, with that provider's declared ceiling (ADR-0023,
// ADR-0077).
func (s *serving) rateAccounts(context.Context) ([]lease.Account, error) {
	var out []lease.Account
	for _, a := range s.snapshot.Load().Accounts() {
		if a.Provider() != gmailProvider {
			continue
		}
		out = append(out, lease.Account{ID: a.ID(), Ceiling: gmail.Profile{}.BudgetPerSecond()})
	}
	return out, nil
}

// idsOf returns the identifiers of a snapshot's accounts, in order.
func idsOf(snapshot *accountload.Snapshot) []string {
	var ids []string
	for _, a := range snapshot.Accounts() {
		ids = append(ids, a.ID())
	}
	return ids
}

// refuseEnvironment returns an error while MCPGODEBUG, PGPASSWORD or PGSSLPASSWORD is set, even to
// an empty value. The MCP SDK reads MCPGODEBUG to switch behaviour this root relies on, sessions in
// a stateless server among them. pgx reads the two passwords on every parse, and either would win
// over the mounted password file.
func refuseEnvironment(environ []string) error {
	for _, entry := range environ {
		if name, _, _ := strings.Cut(entry, "="); name == "MCPGODEBUG" {
			return errors.New("MCPGODEBUG is set, and the mediator runs the MCP SDK only as it ships (ADR-0086)")
		}
	}
	return dbconnect.RefusePasswordVariables(environ)
}

// markReadyWhenServable marks the mediator ready once the TLS key pair, unless an ingress in front
// terminates TLS, and the bearer token load, so a pod whose mounts cannot serve a client never
// reports ready. They are read again on each handshake and each request.
func markReadyWhenServable(ready *readiness.State, c Configuration) error {
	if !c.TLSAtIngress {
		if _, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey); err != nil {
			return fmt.Errorf("loading the TLS key pair: %w", err)
		}
	}
	token, err := os.ReadFile(c.TokenFile)
	if err != nil {
		return fmt.Errorf("reading the bearer token: %w", err)
	}
	if strings.TrimSpace(string(token)) == "" {
		return errors.New("the bearer token file holds no token")
	}
	ready.MarkReady()
	return nil
}

// serveSurface serves the client surface on ln until it is shut down, over TLS unless srv has no TLS
// configuration, which only a declared ingress in front leaves it without (ADR-0087).
func serveSurface(srv *http.Server, ln net.Listener) error {
	if srv.TLSConfig == nil {
		return stopped(srv.Serve(ln))
	}
	return stopped(srv.ServeTLS(ln, "", ""))
}

// serveProbes serves the probes and the metrics endpoint on ln until it is shut down.
func serveProbes(srv *http.Server, ln net.Listener) error {
	return stopped(srv.Serve(ln))
}

// stopped returns nil for a server that stopped because it was shut down.
func stopped(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// surface returns the client surface, the API root and the MCP root generated from registry, behind
// the bearer check. Anything else the surface is asked for is not found, after the bearer check.
// Every response carries Cache-Control: no-store and no ETag, so no response a gate decided outlives
// a change in its decision (ADR-0087). Both roots and the checks in front of them log through logger.
func surface(registry service.Registry, tokenFile string, logger *slog.Logger) (http.Handler, error) {
	apiRoot, err := api.Handler(registry, logger)
	if err != nil {
		return nil, err
	}
	roots := http.NewServeMux()
	roots.Handle("/api/", apiRoot)
	roots.Handle("/mcp", mcp.Handler(registry, version(), logger))
	return noStore(logger, bearer(tokenFile, logger, roots)), nil
}

// noStore marks every response uncacheable, before its handler runs, so a response its handler
// never writes carries the mark too, and again as its header is written, removing any ETag, so a
// handler's own caching headers do not survive.
func noStore(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w, logger: logger}, r)
	})
}

// noStoreWriter sets the caching headers when the response's header is written, so a handler
// setting its own cannot override them.
type noStoreWriter struct {
	http.ResponseWriter
	logger      *slog.Logger
	wroteHeader bool
}

func (w *noStoreWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Del("ETag")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *noStoreWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush passes a flush through, for the MCP transport's streamed responses. A failed flush means the
// client went away, which leaves the operator nothing to act on, so it is detail.
func (w *noStoreWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		w.logger.Debug("flushing a response failed", "error", err)
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *noStoreWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// bearer admits a request to next only when it carries the bearer token held in tokenFile. The file
// is read on each request. A file that cannot be read or holds no token admits nothing.
func bearer(tokenFile string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
		if err != nil {
			logger.ErrorContext(r.Context(), "reading the bearer token failed", "error", err)
		}
		want = []byte(strings.TrimSpace(string(want)))
		scheme, got, _ := strings.Cut(r.Header.Get("Authorization"), " ")
		if len(want) == 0 || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tlsConfig returns the client surface's TLS configuration, which reads the key pair from its
// mounted files on each handshake, or none when an ingress in front terminates TLS.
func tlsConfig(certFile, keyFile string, atIngress bool) *tls.Config {
	if atIngress {
		return nil
	}
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

// probes returns the operational endpoints. /healthz answers 200 while the process serves, /readyz
// answers 200 only while ready reports ready and 503 otherwise, and /metrics serves the process's
// registry.
func probes(ready *readiness.State, metrics *prometheus.Registry, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, logger, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Ready() {
			answer(w, r, logger, http.StatusServiceUnavailable, "not ready")
			return
		}
		answer(w, r, logger, http.StatusOK, "ready")
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	return mux
}

// answer writes a plain-text probe answer. A failed write means the prober went away, which leaves
// the operator nothing to act on, so it is detail.
func answer(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprintln(w, text); err != nil {
		logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}

// version returns the mediator's module version, as the build recorded it.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// The series counting body requests, served and denied, a denial told apart by the stage that
// decided it and its reason (ADR-0036, ADR-0002).
const (
	bodiesServedName = "mediated_mailbox_mediate_bodies_served_total"
	bodiesDeniedName = "mediated_mailbox_mediate_bodies_denied_total"
)

// newBodies returns what the body operation needs beyond the index, with the opener of its provider
// sessions. Its policy is loaded for each request through served, its provider calls go through the
// account sessions it holds, whose sources and ports connect builds, the serve-time pattern check runs
// scanner, and its outcomes are counted on series
// metrics serves. Each provider call is bounded by timeout.
func newBodies(served *serving, connect session.Connector, scanner scan.Scanner, timeout time.Duration, metrics prometheus.Registerer) (service.Bodies, *providers, error) {
	leases, err := lease.NewMetrics(metrics)
	if err != nil {
		return service.Bodies{}, nil, err
	}
	servedBodies := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesServedName,
		Help: "The bodies the mediator served, each after its audit row was written.",
	}, []string{"account"})
	deniedBodies := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesDeniedName,
		Help: "The body requests the mediator denied, by the stage that decided it, gate or serve, and the reason.",
	}, []string{"account", "stage", "reason"})
	for _, c := range []prometheus.Collector{servedBodies, deniedBodies} {
		if err := metrics.Register(c); err != nil {
			return service.Bodies{}, nil, err
		}
	}
	p := &providers{served: served, leases: leases, timeout: timeout, sessions: session.New(session.Config{
		Loader: served.loader, DB: served.pool, Logger: served.logger,
		Connectors: map[string]session.Connector{gmailProvider: connect}, Hold: true,
	})}
	return service.Bodies{
		Policy:  served.currentPolicy,
		Open:    p.open,
		Convert: markdown.Convert,
		Literal: markdown.Literal,
		Scanner: scanner,
		Observe: func(account string, o service.Outcome) {
			if o.Stage == "" {
				servedBodies.WithLabelValues(account).Inc()
				return
			}
			deniedBodies.WithLabelValues(account, o.Stage, o.Reason).Inc()
		},
	}, p, nil
}

// gmailSource builds the Gmail token source of an account from its credentials, the OAuth client the
// account connects through and its refresh token (ADR-0106).
func gmailSource(c session.Credentials) *gmail.TokenSource {
	return gmail.NewTokenSource(http.DefaultClient, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
}

// gmailConnector returns the connector that builds a Gmail token source for an account and a Gmail
// adapter over it, counting its requests on series registry serves, which every adapter it builds
// shares.
func gmailConnector(registry prometheus.Registerer) (session.Connector, error) {
	metrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return session.Connector{}, err
	}
	return session.Connect(gmailSource, func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: http.DefaultClient, Tokens: source, Metrics: metrics})
	}), nil
}

// providers opens each body request's session with its account's provider. Its account sessions keep
// one token source per account across requests, so an access token the provider issued serves the
// requests after it rather than each request refreshing its own. An account the snapshot in force
// does not list, that is not connected, or whose provider is not Gmail is not connected, and its
// credential is the one the snapshot opened from its state row and nothing else (ADR-0080, ADR-0106).
type providers struct {
	served   *serving
	sessions *session.Holder
	leases   *lease.Metrics
	// timeout bounds each provider call (ADR-0101).
	timeout time.Duration
}

// open returns a body request's session with the account's provider, the request's unit of work. It
// is called only once the gate has released a message (ADR-0002).
func (p *providers) open(ctx context.Context, account string) (service.Provider, error) {
	opened, err := p.sessions.Open(p.served.loader.Snapshot(), account)
	if err != nil {
		return nil, err
	}
	limiter, err := newLimiter(ctx, p.served.pool, account, p.leases)
	if err != nil {
		return nil, err
	}
	return &provider{p: p, account: account, session: opened, limiter: limiter}, nil
}

// provider is one body request's calls to the account's provider, through its account session.
type provider struct {
	p       *providers
	account string
	session *session.Session
	limiter *lease.Limiter
}

// Body returns the message's body, under a lease in the interactive class.
func (s *provider) Body(ctx context.Context, id string) (mail.MessageBody, error) {
	return session.Call(ctx, s.session, func(ctx context.Context, port mail.Port[context.Context]) (mail.MessageBody, error) {
		return leased(ctx, s.limiter, port, s.account, mail.OpGetMessageBody, 0, s.p.timeout, func(ctx context.Context) (mail.MessageBody, error) {
			return port.GetMessageBody(ctx, id)
		})
	})
}

// Metadata returns the message's metadata, under a lease in the interactive class, or an error
// wrapping mail.ErrNotFound when the provider no longer holds the message.
func (s *provider) Metadata(ctx context.Context, id string) (mail.MessageMetadata, error) {
	read, err := session.Call(ctx, s.session, func(ctx context.Context, port mail.Port[context.Context]) ([]mail.MessageMetadata, error) {
		return leased(ctx, s.limiter, port, s.account, mail.OpGetMessageMetadata, 1, s.p.timeout, func(ctx context.Context) ([]mail.MessageMetadata, error) {
			return port.GetMessageMetadata(ctx, []string{id})
		})
	})
	if err != nil {
		return mail.MessageMetadata{}, err
	}
	if len(read) == 0 {
		return mail.MessageMetadata{}, fmt.Errorf("message %s: %w", id, mail.ErrNotFound)
	}
	return read[0], nil
}

// Done ends the request's unit of work through its account session, which hands the source's current
// refresh token to the loader, naming the adoption that came with the credential the source was built
// from, and records the latest authentication attempt the source reports (ADR-0082, ADR-0089,
// ADR-0097). A failure of either is logged and fails nothing, since the request's answer is already
// decided. The loader keeps a rotated credential whose write-back failed, and the next request hands
// it over again (ADR-0090).
func (s *provider) Done(ctx context.Context) {
	logger := s.p.served.logger
	handedOver, recorded := s.session.End(ctx)
	if handedOver != nil {
		logger.Error("handing the account's credential over failed", "account", s.account, "error", handedOver)
	}
	if recorded != nil {
		logger.Error("recording the last authentication attempt failed", "account", s.account, "error", recorded)
	}
}

// leased makes one call of port's operation op for the account under a lease in the interactive
// class, sized by what port's rate profile says a call naming messages messages costs, and tells the
// limiter how the call went (ADR-0023, ADR-0024, ADR-0025). A failure to tell the limiter fails the
// call, since a controller that never hears of a throttle would not slow down. The call has timeout
// to answer, and one that has not answered by then while ctx is still live is the provider's failure
// (ADR-0101). The lease's own wait is not bounded by it.
func leased[T any](ctx context.Context, limiter *lease.Limiter, port mail.Port[context.Context], account string, op mail.Operation, messages int,
	timeout time.Duration, call func(context.Context) (T, error),
) (T, error) {
	var zero T
	cost := port.RateProfile().Cost(mail.ProviderOp{Operation: op, Messages: messages})
	if _, err := limiter.Acquire(ctx, account, ratecore.Interactive, cost.Weight); err != nil {
		return zero, fmt.Errorf("leasing the call: %w", err)
	}
	start := time.Now()
	bounded, cancel := context.WithTimeout(ctx, timeout)
	out, err := call(bounded)
	if err != nil && errors.Is(bounded.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		err = fmt.Errorf("%w: no answer within %s: %w", mail.ErrProvider, timeout, err)
	}
	cancel()
	var told error
	switch signal, throttled := mail.Throttled(err); {
	case err == nil:
		told = limiter.Succeeded(ctx, account, cost, time.Since(start))
	case throttled:
		told = limiter.Throttled(ctx, account, signal)
	case errors.Is(err, mail.ErrProvider):
		told = limiter.ServerErrored(ctx, account)
	}
	if told != nil {
		return zero, errors.Join(err, fmt.Errorf("telling the rate limiter how the call went: %w", told))
	}
	return out, err
}

// newLimiter returns the rate limiter a body request spends from for a Gmail account, at the lower
// target the account's state row sets, if it sets one (ADR-0024). It is built for each request, so a
// target changed while the mediator runs applies from the next request.
func newLimiter(ctx context.Context, db tx.Beginner, account string, metrics *lease.Metrics) (*lease.Limiter, error) {
	targets := map[string]float64{}
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		target, err := accountstate.New(t).LoweredTarget(ctx, account)
		if err != nil {
			return err
		}
		if target.Valid {
			targets[account] = float64(target.Float32)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the lowered target of account %s: %w", account, err)
	}
	limiter, err := lease.NewWithTargets(db, gmail.Profile{}.BudgetPerSecond(), targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
}
