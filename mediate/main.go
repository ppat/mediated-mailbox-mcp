// Command mediate is the mediator, published as mediated-mailbox-mediate.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
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
package main

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
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/settings"
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
}

// defaults are the mediator's defaults. The user is the mediator's own runtime role (ADR-0075), and
// the TLS mode is the one that fails closed. The TLS, token and key files have no default, since a
// default path assumes the environment.
func defaults() Configuration {
	return Configuration{
		Listen:                ":8443",
		ProbeListen:           ":8080",
		AccountReloadInterval: time.Minute,
		Database:              dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_mediate", SSLMode: "verify-full"},
	}
}

// validate refuses a configuration the mediator cannot serve with. The TLS files are required unless
// an ingress is declared to terminate TLS, and refused when one is. The reload interval is positive.
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
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Environ(), slog.Default())
	stop()
	if err != nil {
		slog.Error("the mediator stopped", "error", err)
		os.Exit(1)
	}
}

// run loads the configuration and the keyring, connects to the database, loads the accounts and
// their policy, and serves until ctx ends. It logs the effective configuration first, each value
// with the layer that set it (ADR-0078). The keyring is loaded before any connection is made, so a
// public key matching none of the private keys refuses the start (ADR-0088).
func run(ctx context.Context, args, environ []string, logger *slog.Logger) error {
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
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	c := loaded.Config
	if err := validate(c); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
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
	served, err := newServing(pool, keys, metrics, logger)
	if err != nil {
		return err
	}
	registry, err := service.NewRegistry(nil, service.Operations(sources(pool, served))...)
	if err != nil {
		return err
	}
	served.registry = registry
	if err := served.reload(ctx); err != nil {
		return err
	}
	return serve(ctx, c, served, metrics, logger)
}

// serve listens, reloads the account snapshot on its interval and serves the registry until ctx
// ends.
func serve(ctx context.Context, c Configuration, served *serving, metrics *prometheus.Registry, logger *slog.Logger) error {
	registry := served.registry
	var ready readiness.State

	surfaceListener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return err
	}
	probeListener, err := net.Listen("tcp", c.ProbeListen)
	if err != nil {
		return errors.Join(err, surfaceListener.Close())
	}
	surfaceHandler, err := surface(registry, c.TokenFile)
	if err != nil {
		return errors.Join(err, surfaceListener.Close(), probeListener.Close())
	}
	surfaceServer := &http.Server{
		Handler:           surfaceHandler,
		TLSConfig:         tlsConfig(c.TLSCert, c.TLSKey, c.TLSAtIngress),
		ReadHeaderTimeout: 10 * time.Second,
	}
	probeServer := &http.Server{
		Handler:           probes(&ready, metrics),
		ReadHeaderTimeout: 10 * time.Second,
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

// sources are what the read operations read from, the pool, the accounts and policy the serving state
// holds in force, the classifier's lookups and the clock.
func sources(pool *pgxpool.Pool, served *serving) service.Sources {
	return service.Sources{
		DB:       pool,
		Accounts: served.accounts,
		Policy:   served.policy,
		Lookups:  lookups(),
		Now:      time.Now,
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
// account snapshot and serves its accounts. The policy is loaded again whenever the accounts differ
// from the ones it was last loaded for, and an account whose rules no load has read restricts every
// sender (ADR-0090, ADR-0041).
type serving struct {
	loader  *accountload.Loader
	pool    *pgxpool.Pool
	metrics *prometheus.Registry
	logger  *slog.Logger
	// policies is the policy loader, built at the first reload that serves an account. Calls read it
	// while a reload may be building it.
	policies atomic.Pointer[policyload.Loader]
	registry service.Registry
	// snapshot is the account snapshot the registry serves.
	snapshot atomic.Pointer[accountload.Snapshot]
	// policyFor are the accounts the policy was last loaded for, which only reload reads and writes.
	policyFor []string
}

// newServing returns the serving state, serving no account until its first reload, and registers on
// metrics F3's collector of each served account's rate-state series, which the mediator carries
// because it runs continuously (ADR-0077).
func newServing(pool *pgxpool.Pool, keys *open.Keyring, metrics *prometheus.Registry, logger *slog.Logger) (*serving, error) {
	s := &serving{loader: accountload.New(pool, keys, logger), pool: pool, metrics: metrics, logger: logger}
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
// The policy is loaded again whenever the accounts differ from the ones it was last loaded for. A
// policy load that fails is returned, and the snapshot's accounts are served all the same, so an
// account the snapshot drops is dropped. The policy loader keeps its active snapshot, so an account it
// read before keeps that policy, and an account it never read gets the policy that restricts every
// sender (ADR-0041). The next reload loads the policy again.
func (s *serving) reload(ctx context.Context) error {
	if err := s.loader.Load(ctx); err != nil {
		return fmt.Errorf("loading the accounts: %w", err)
	}
	next := s.loader.Snapshot()
	ids := idsOf(next)
	var failed error
	if !slices.Equal(ids, s.policyFor) || s.policies.Load() == nil {
		if failed = s.loadPolicy(ctx, ids); failed == nil {
			s.policyFor = ids
		}
	}
	s.snapshot.Store(next)
	s.registry.Serve(ids)
	s.logger.Info("accounts served", "accounts", len(ids))
	return failed
}

// loadPolicy loads the policy of accounts through the shared policy loader, which reads each
// account's policy in a transaction of that account. With no account there is no policy to load.
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
		policies = built
		s.policies.Store(policies)
	} else if err := policies.SetAccounts(accounts); err != nil {
		return err
	}
	if err := policies.Reload(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	s.logger.Info("policy loaded", "accounts", len(accounts))
	return nil
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
// a change in its decision (ADR-0087).
func surface(registry service.Registry, tokenFile string) (http.Handler, error) {
	apiRoot, err := api.Handler(registry)
	if err != nil {
		return nil, err
	}
	roots := http.NewServeMux()
	roots.Handle("/api/", apiRoot)
	roots.Handle("/mcp", mcp.Handler(registry, version()))
	return noStore(bearer(tokenFile, roots)), nil
}

// noStore marks every response uncacheable, before its handler runs, so a response its handler
// never writes carries the mark too, and again as its header is written, removing any ETag, so a
// handler's own caching headers do not survive.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
	})
}

// noStoreWriter sets the caching headers when the response's header is written, so a handler
// setting its own cannot override them.
type noStoreWriter struct {
	http.ResponseWriter
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

// Flush passes a flush through, for the MCP transport's streamed responses.
func (w *noStoreWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		slog.Warn("flushing a response failed", "error", err)
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *noStoreWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// bearer admits a request to next only when it carries the bearer token held in tokenFile. The file
// is read on each request. A file that cannot be read or holds no token admits nothing.
func bearer(tokenFile string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
		if err != nil {
			slog.ErrorContext(r.Context(), "reading the bearer token failed", "error", err)
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
func probes(ready *readiness.State, metrics *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Ready() {
			answer(w, r, http.StatusServiceUnavailable, "not ready")
			return
		}
		answer(w, r, http.StatusOK, "ready")
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	return mux
}

// answer writes a plain-text probe answer.
func answer(w http.ResponseWriter, r *http.Request, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprintln(w, text); err != nil {
		slog.WarnContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}

// version returns the mediator's module version, as the build recorded it.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
