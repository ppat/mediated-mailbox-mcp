// Package app is the API server's entry package. It exports the API server's section of the
// configuration and its assembly, which takes what the API server needs as values, starts nothing and
// returns what it provides, and its composition root, Run, which hands the host the API server and
// the configuration it reads (ADR-0040, ADR-0138). api/main.go calls Run. It sits outside internal, so
// a composition root other than the API server's own can compose it. Every other package of this
// deployable sits under internal, apart from importtarget, which exists only under the banproof tag.
//
// The API server's handler is the client surface, the API root under /api/ and the MCP root at /mcp,
// and every request to it passes the bearer check before it reaches either root (ADR-0030,
// ADR-0087). The host serves it over TLS when the configuration names a certificate and a key, and
// over plain HTTP when it names neither (ADR-0127), and serves the probes and the metrics endpoint
// over plain HTTP on a listener of their own, because a platform's healthcheck and scraper carry no
// bearer token (ADR-0051). The API server is ready once its accounts and their policy have loaded and
// while its bearer token file holds a token.
//
// Its database connection is built from the configuration's database section, with the password in
// the mounted file the section names (ADR-0078, ADR-0079). The accounts it serves live in the database
// and never in its configuration (ADR-0080). They reach the registry, the policy and the rate-state
// collector through the process's execution context, whose account snapshot its assembly loads and
// whose reload, on the interval the configuration sets, is a task the host runs (ADR-0090). The
// registry refuses every call naming an account the snapshot in force does not list. The bearer token is a mounted file, read again on each
// request, so a rotated file takes effect without a restart (ADR-0079, ADR-0080). The API server
// refuses to start while MCPGODEBUG is set, because the MCP SDK reads it to switch its behaviour
// (ADR-0086).
package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/reload"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// gmailProvider is the provider an account served through the Gmail adapter names in accounts.
const gmailProvider = "gmail"

// Configuration is the API server's root configuration type, loaded by the configuration library
// (ADR-0078). It composes the process's sections, the API server's own section and the sections it
// shares with the other deployables, the process's and its own inline, so every key sits at the top of
// the tree. Its fields are pinned by the test beside this file, so a new value is a visible change. No
// account and no provider credential is configuration. Both come from the database (ADR-0080).
type Configuration struct {
	host.Process `yaml:",inline"`
	host.Serving `yaml:",inline"`
	Section      `yaml:",inline"`
	Database     dbconnectcore.Config  `yaml:"database"`
	Credential   credentialcore.Config `yaml:"credential"`
	// Scanner is the scanner's section, the same section backfill reads, whose pattern tier the
	// serve-time pattern check runs (ADR-0002, ADR-0005). No value in it switches the check off.
	Scanner scan.Config `yaml:"scanner"`
}

// Section is the API server's own section of the configuration, the values only it reads.
type Section struct {
	// TokenFile names the mounted file holding the bearer token clients present.
	TokenFile string `yaml:"token_file" settings:"required"`
	// AccountReloadInterval is how often the execution context the API server's root builds reloads
	// its account snapshot and their policy (ADR-0090).
	AccountReloadInterval time.Duration `yaml:"account_reload_interval"`
	// ProviderTimeout bounds each provider call a body request makes, after which the call is the
	// provider's failure (ADR-0101).
	ProviderTimeout time.Duration `yaml:"provider_timeout"`
}

// maxProviderTimeout is the longest provider timeout the configuration accepts.
const maxProviderTimeout = 5 * time.Minute

// leaseWait bounds how long one body request waits for its leases, counted from the request's
// arrival, so no request waits without limit whatever revision its client speaks. A request no lease
// comes for is answered within it, before Claude Code's own 60-second timer on a request's first
// response byte gives up on the call (ADR-0130).
const leaseWait = 45 * time.Second

// scannerSection is the scanner's section of the configuration, whose revision the serve-time
// pattern check runs under (ADR-0078).
const scannerSection = "scanner"

// DefaultSection is the API server's own section's defaults. The token file has no default, since a
// default path assumes the environment.
func DefaultSection() Section {
	return Section{AccountReloadInterval: time.Minute, ProviderTimeout: 30 * time.Second}
}

// defaults are the API server's defaults. The user is the API server's own runtime role (ADR-0118), and
// the TLS mode is the one that fails closed, and the pool size is the driver's own default. The key
// files have no default, since a default path assumes the environment.
func defaults() Configuration {
	return Configuration{
		Process:  host.DefaultProcess(),
		Serving:  host.DefaultServing(),
		Section:  DefaultSection(),
		Database: dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_api", SSLMode: "verify-full", PoolSize: dbconnect.DefaultPoolSize()},
		Scanner:  scan.DefaultConfig(),
	}
}

// ValidateSection refuses an API server section the API server cannot serve with. The token file is
// named, the reload interval is positive, and the provider timeout positive and at most five minutes.
func ValidateSection(s Section) error {
	if strings.TrimSpace(s.TokenFile) == "" {
		return errors.New("token_file is empty")
	}
	if s.AccountReloadInterval <= 0 {
		return fmt.Errorf("account_reload_interval %s is not positive", s.AccountReloadInterval)
	}
	if s.ProviderTimeout <= 0 || s.ProviderTimeout > maxProviderTimeout {
		return fmt.Errorf("provider_timeout %s is not positive and at most %s", s.ProviderTimeout, maxProviderTimeout)
	}
	return nil
}

// validate refuses a configuration the API server cannot serve with, past the process's sections,
// which the host validates.
func validate(c Configuration) error {
	if err := ValidateSection(c.Section); err != nil {
		return err
	}
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

// Mount is where the API server's handler is served on a process's application server, the API root
// and the MCP root (ADR-0030, ADR-0087).
var Mount = []string{"/api/", "/mcp"}

// Run is the API server's process. The host loads the configuration, and the API server is assembled
// from it, the keyring loaded before any connection is made, so a public key matching none of the
// private keys refuses the start (ADR-0088), and its accounts and their policy loaded before it serves
// (ADR-0138). The process's execution context, built once over the API server's pool, reloads in a
// task of the process (ADR-0090).
//
// logger is the logger main.go built over level, at info until the host sets level from log_level
// once the effective configuration is written, so every logger derived from logger follows it
// (ADR-0122). stdout answers --help and --version, and stderr is where the start banner goes,
// standard error in main.go, so standard output carries only the logger's JSON records otherwise.
func Run(ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, host.Spec[Configuration]{
		Name:     "mediated-mailbox-api",
		Defaults: defaults(),
		Process:  func(c Configuration) host.Process { return c.Process },
		Serving:  func(c Configuration) host.Serving { return c.Serving },
		Validate: validate,
		Components: []host.Component[Configuration]{{
			Name: "api", Mount: Mount, RefuseEnvironment: RefuseEnvironment, Assemble: fromConfiguration,
		}},
	})
}

// fromConfiguration builds what the API server needs from the configuration, the scanner from its
// section under the section's revision, the keyring, the pool connecting as the API server's role and
// the process's execution context over that pool, and assembles the API server from them. The
// context's reload is the process's task.
func fromConfiguration(ctx context.Context, provided host.Provided[Configuration]) (host.Parts, error) {
	c := provided.Loaded.Config
	scanner, err := scan.New(c.Scanner, provided.Loaded.Revisions[scannerSection])
	if err != nil {
		return host.Parts{}, fmt.Errorf("validating the configuration: scanner: %w", err)
	}
	keys, err := open.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles...)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the keyring: %w", err)
	}
	pool, err := provided.OpenPool(ctx, c.Database, provided.Registerer)
	if err != nil {
		return host.Parts{}, err
	}
	ec, err := newContext(pool, keys, scanner, c.Section, provided.Registerer, provided.Logger)
	if err != nil {
		return host.Parts{}, err
	}
	parts, err := Assemble(ctx, Needs{
		Section: c.Section, Pool: pool, Context: ec, Client: http.DefaultClient,
		Registerer: provided.Registerer, Logger: provided.Logger, Start: provided.Start,
	})
	if err != nil {
		return host.Parts{}, err
	}
	parts.Tasks = append(parts.Tasks, ContextTask(ec))
	return parts, nil
}

// newContext returns the execution context of a process that hosts the API server alone, over the
// API server's pool and so under its role, reloading on its account reload interval (ADR-0090,
// ADR-0118).
func newContext(pool *pgxpool.Pool, keys *open.Keyring, scanner scan.Scanner, s Section, reg prometheus.Registerer, logger *slog.Logger) (*assembly.Context, error) {
	return assembly.New(assembly.Needs{
		Pool: pool, Keys: keys, ThroughClient: []string{gmailProvider}, Scanner: scanner, Lookups: lookups(),
		ReloadInterval: s.AccountReloadInterval, Registerer: reg, Logger: logger,
	})
}

// ContextTask is the task that reloads the process's execution context on its interval, which the
// root that builds the context hands the host once.
func ContextTask(ec *assembly.Context) host.Task {
	return host.Task{Name: "execution context reload", Run: ec.Run}
}

// Needs are what the API server is assembled from.
type Needs struct {
	Section Section
	// Pool connects as the API server's runtime role, which its own statements, its leases and its
	// recording of authentication attempts run under (ADR-0118).
	Pool *pgxpool.Pool
	// Context is the process's execution context, its accounts, their sessions' sources, their policy
	// and the scanner the serve-time pattern check runs (ADR-0005, ADR-0090).
	Context *assembly.Context
	// Client is the HTTP client the provider adapters and their token sources call through.
	Client *http.Client
	// Registerer is where every series of the API server is registered (ADR-0076).
	Registerer prometheus.Registerer
	Logger     *slog.Logger
	// Start starts the goroutine a body request's policy load runs on, the host's in a process
	// (ADR-0138).
	Start host.Starter
}

// Assemble builds the API server from n, has the execution context load its accounts and their
// policy, and returns its parts, the client surface behind the bearer check and the readiness check
// of the bearer token. It serves the accounts of every reload of the context. It starts no goroutine
// of its own. Its policy loads run
// on goroutines n.Start starts, under a context that outlives the requests, so they end when the
// process stops and never because a client stopped waiting (ADR-0138). A load that fails, or a bearer
// token file that cannot be read or holds no token, fails the assembly, so a process that cannot
// serve a client never starts (ADR-0090, ADR-0127).
func Assemble(ctx context.Context, n Needs) (host.Parts, error) {
	connect, err := gmailConnector(n.Registerer, n.Client)
	if err != nil {
		return host.Parts{}, err
	}
	a, err := assemble(ctx, n, connect, leaseWait)
	if err != nil {
		return host.Parts{}, err
	}
	return a.parts, nil
}

// assembled is the API server built, its serving state, the opener of its body requests' provider
// sessions and its parts.
type assembled struct {
	served    *serving
	providers *providers
	parts     host.Parts
}

// assemble is Assemble with the connector its accounts' provider sessions connect through and the
// bound on a body request's lease waits, which the tests and the live agent's exercise harness give
// over the provider fake, so each of them builds the API server as Assemble does.
func assemble(ctx context.Context, n Needs, connect session.Connector, wait time.Duration) (*assembled, error) {
	served, err := newServing(n.Start, n.Pool, n.Context, n.Registerer, n.Logger)
	if err != nil {
		return nil, err
	}
	bodies, opener, err := newBodies(served, connect, n.Context.Scanner(), n.Section.ProviderTimeout, wait, n.Registerer)
	if err != nil {
		return nil, err
	}
	ops, err := timed(service.Operations(sources(n.Pool, served, bodies)), n.Registerer)
	if err != nil {
		return nil, err
	}
	registry, err := service.NewRegistry(nil, ops...)
	if err != nil {
		return nil, err
	}
	served.registry = registry
	n.Context.OnReload(served.serve)
	if err := served.reload(ctx); err != nil {
		return nil, err
	}
	p, err := parts(served, n.Section, n.Logger)
	if err != nil {
		return nil, err
	}
	return &assembled{served: served, providers: opener, parts: p}, nil
}

// parts returns the API server's parts over served, whose registry is built and whose first reload
// has run.
func parts(served *serving, s Section, logger *slog.Logger) (host.Parts, error) {
	handler, err := surface(served.registry, s.TokenFile, logger)
	if err != nil {
		return host.Parts{}, err
	}
	if err := tokenReadable(s.TokenFile); err != nil {
		return host.Parts{}, err
	}
	return host.Parts{
		Handler: handler,
		Ready: []host.Check{{Name: "bearer token", Check: func(context.Context) error {
			return tokenReadable(s.TokenFile)
		}}},
	}, nil
}

// operationDurationName is the series timing each operation of the client surface (ADR-0125).
const operationDurationName = "mediated_mailbox_api_operation_duration_seconds"

// timed returns ops with each operation's handler timed on a series metrics serves, labelled by the
// operation's name, so every call either root makes is timed from the handler's start to its return,
// after the registry's account check. The service layer is unchanged and imports no metrics library
// (ADR-0125).
func timed(ops []service.Operation, metrics prometheus.Registerer) ([]service.Operation, error) {
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    operationDurationName,
		Help:    "How long each operation of the client surface took, from its handler's start to its return, by operation.",
		Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"operation"})
	if err := metrics.Register(duration); err != nil {
		return nil, err
	}
	out := make([]service.Operation, 0, len(ops))
	for _, op := range ops {
		handle, observed := op.Handle, duration.WithLabelValues(op.Name)
		op.Handle = func(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
			start := time.Now()
			defer func() { observed.Observe(time.Since(start).Seconds()) }()
			return handle(ctx, account, input)
		}
		out = append(out, op)
	}
	return out, nil
}

// sources are what the operations read from, the pool, the accounts and policy the serving state
// holds in force, the clock, and what the body operation needs beyond them.
func sources(pool *pgxpool.Pool, served *serving, bodies service.Bodies) service.Sources {
	return service.Sources{
		DB:       pool,
		Accounts: served.accounts,
		Policy:   served.policy,
		Now:      time.Now,
		Bodies:   bodies,
	}
}

// lookups are the classifier's domain functions, the UTS #46 lookup profile and the public-suffix
// list, which the policy loader prepares each account's policy with for the pure classifier.
func lookups() classify.Lookups {
	return classify.Lookups{
		ToUnicode:   idna.Lookup.ToUnicode,
		ToASCII:     idna.Lookup.ToASCII,
		Registrable: publicsuffix.EffectiveTLDPlusOne,
	}
}

// serving is the set of accounts the API server serves and their policy, which the process's
// execution context holds. Each reload of the context serves its snapshot's accounts, and an account
// whose rules no load has read restricts every sender (ADR-0090, ADR-0041). A body request has the
// policy loaded again before it decides, through the coalesced load (ADR-0099).
type serving struct {
	context *assembly.Context
	pool    *pgxpool.Pool
	logger  *slog.Logger
	// fresh runs the policy loads, each started after every caller it answers asked (ADR-0099).
	fresh    *reload.Coalesced
	registry service.Registry
}

// newServing returns the serving state over the execution context ec, and registers on metrics F3's
// collector of each served account's rate-state series, read through pool, which the API server
// carries because it runs continuously (ADR-0077). Its policy loads run on goroutines start starts,
// so they end when the API server stops and never because a client stopped waiting.
func newServing(start host.Starter, pool *pgxpool.Pool, ec *assembly.Context, metrics prometheus.Registerer, logger *slog.Logger) (*serving, error) {
	s := &serving{context: ec, pool: pool, logger: logger}
	s.fresh = reload.New(reload.Starter(start), ec.ReloadPolicy)
	if err := metrics.Register(lease.NewCollector(pool, s.rateAccounts)); err != nil {
		return nil, err
	}
	return s, nil
}

// reload has the execution context reload its account snapshot and the policy of its accounts, which
// serves them. A snapshot read that fails keeps the accounts served and their policy, and returns the
// error, which the start treats as fatal. A read listing no account serves none, and an account a
// read no longer lists is dropped (ADR-0090). A policy load that fails is returned, and the snapshot's
// accounts are served all the same, so an account the snapshot drops is dropped. The policy loader
// keeps its active snapshot, so an account it read before keeps that policy, and an account it never
// read gets the policy that restricts every sender (ADR-0041). The next reload loads the policy again.
func (s *serving) reload(ctx context.Context) error {
	return s.context.Reload(ctx)
}

// serve serves the accounts of the snapshot the execution context's latest reload left in force.
func (s *serving) serve(context.Context) {
	ids := idsOf(s.context.Snapshot())
	s.registry.Serve(ids)
	s.logger.Info("accounts served", "accounts", len(ids))
}

// currentPolicy has the policy loaded by a load that starts after the call, and returns the account's
// policy from the snapshot then active (ADR-0099). A load that fails leaves the active valid policy,
// whose failure the loader alarms on (ADR-0041). A caller that stops waiting gets the policy that
// restricts every sender, so a decision is never made against a policy the load did not reach.
func (s *serving) currentPolicy(ctx context.Context, account string) classify.Policy {
	if err := s.fresh.Load(ctx); err != nil {
		if ctx.Err() != nil {
			return classify.Policy{}
		}
		s.logger.Error("the policy was not loaded, so the active policy decides", "error", err)
	}
	return s.policy(account)
}

// accounts returns the accounts served with their providers, for the accounts listing.
func (s *serving) accounts() []service.Account {
	var out []service.Account
	for _, a := range s.context.Snapshot().Accounts() {
		out = append(out, service.Account{ID: a.ID(), Provider: a.Provider()})
	}
	return out
}

// policy returns the policy an account's messages are decided against. With no policy loaded it is
// the policy that restricts every sender.
func (s *serving) policy(account string) classify.Policy {
	return s.context.Policy(account)
}

// rateAccounts returns the accounts the rate-state collector reports on, every served account whose
// provider the API server has a rate profile for, with that provider's declared ceiling (ADR-0023,
// ADR-0077).
func (s *serving) rateAccounts(context.Context) ([]lease.Account, error) {
	var out []lease.Account
	for _, a := range s.context.Snapshot().Accounts() {
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

// RefuseEnvironment returns an error while MCPGODEBUG is set, even to an empty value. The MCP SDK
// reads it when the process starts, to switch behaviour the API server relies on, sessions in a
// stateless server among them, so whatever shares the API server's process runs under it too
// (ADR-0086).
func RefuseEnvironment(environ []string) error {
	for _, entry := range environ {
		if name, _, _ := strings.Cut(entry, "="); name == "MCPGODEBUG" {
			return errors.New("MCPGODEBUG is set, and the API server runs the MCP SDK only as it ships (ADR-0086)")
		}
	}
	return nil
}

// tokenReadable refuses a bearer token file that cannot be read or holds no token, which admits no
// client. The token is read again on each request.
func tokenReadable(tokenFile string) error {
	token, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
	if err != nil {
		return fmt.Errorf("reading the bearer token: %w", err)
	}
	if strings.TrimSpace(string(token)) == "" {
		return errors.New("the bearer token file holds no token")
	}
	return nil
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
	roots.Handle("/mcp", mcp.Handler(registry, version.Number, logger))
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

// The series counting body requests, served and denied, a denial told apart by the stage that
// decided it and its reason (ADR-0036, ADR-0002).
const (
	bodiesServedName = "mediated_mailbox_api_bodies_served_total"
	bodiesDeniedName = "mediated_mailbox_api_bodies_denied_total"
)

// processingName is the series timing each body's conversion, under the name and help the worker's
// job kinds time conversions and scans under. The API server's serve-time pattern check runs inside the
// release decision's pure core and is not timed (ADR-0125).
const processingName = "mediated_mailbox_body_processing_duration_seconds"

// newBodies returns what the body operation needs beyond the index, with the opener of its provider
// sessions. Its policy is loaded for each request through served, its provider calls go through the
// account sessions it holds, whose sources and ports connect builds, the serve-time pattern check runs
// scanner, and its outcomes are counted on series
// metrics serves, which also times each conversion. Each provider call is bounded by timeout, and
// each request's waits for its leases together by wait.
func newBodies(served *serving, connect session.Connector, scanner scan.Scanner, timeout, wait time.Duration, metrics prometheus.Registerer) (service.Bodies, *providers, error) {
	leases, err := lease.NewMetrics(metrics)
	if err != nil {
		return service.Bodies{}, nil, err
	}
	servedBodies := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesServedName,
		Help: "The bodies the API server served, each after its audit row was written.",
	}, []string{"account"})
	deniedBodies := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesDeniedName,
		Help: "The body requests the API server denied, by the stage that decided it, gate or serve, and the reason.",
	}, []string{"account", "stage", "reason"})
	processing := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    processingName,
		Help:    "How long each body's conversion to Markdown and its scan took, by step.",
		Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"step"})
	for _, c := range []prometheus.Collector{servedBodies, deniedBodies, processing} {
		if err := metrics.Register(c); err != nil {
			return service.Bodies{}, nil, err
		}
	}
	converted := processing.WithLabelValues("convert")
	p := &providers{
		served: served, leases: leases, timeout: timeout, wait: wait,
		sessions: served.context.Sessions(map[string]session.Connector{gmailProvider: connect}, served.pool),
	}
	return service.Bodies{
		Policy: served.currentPolicy,
		Open:   p.open,
		Render: func(body mail.MessageBody) (string, error) {
			start := time.Now()
			defer func() { converted.Observe(time.Since(start).Seconds()) }()
			return markdown.Render(body)
		},
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

// gmailSource returns what builds the Gmail token source of an account from its credentials, the OAuth
// client the account connects through and its refresh token, calling through client (ADR-0106).
func gmailSource(client *http.Client) func(session.Credentials) *gmail.TokenSource {
	return func(c session.Credentials) *gmail.TokenSource {
		return gmail.NewTokenSource(client, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
	}
}

// gmailConnector returns the connector that builds a Gmail token source for an account and a Gmail
// adapter over it, both calling through client, counting its requests on series registry serves,
// which every adapter it builds shares.
func gmailConnector(registry prometheus.Registerer, client *http.Client) (session.Connector, error) {
	metrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return session.Connector{}, err
	}
	return session.Connect(gmailSource(client), func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: client, Tokens: source, Metrics: metrics})
	}), nil
}

// providers opens each body request's session with its account's provider. Its account sessions are
// the execution context's, which keeps one token source per account in the process across requests,
// so an access token the provider issued serves the requests after it rather than each request
// refreshing its own. An account the snapshot in force
// does not list, that is not connected, or whose provider is not Gmail is not connected, and its
// credential is the one the snapshot opened from its state row and nothing else (ADR-0080, ADR-0106).
type providers struct {
	served   *serving
	sessions *session.Holder
	leases   *lease.Metrics
	// timeout bounds each provider call (ADR-0101).
	timeout time.Duration
	// wait bounds a request's waits for its leases, counted from its arrival (ADR-0130).
	wait time.Duration
}

// open returns a body request's session with the account's provider, the request's unit of work,
// whose waits for its leases end p.wait after the request arrived. It is called only once the gate
// has released a message (ADR-0002).
func (p *providers) open(ctx context.Context, account string, arrived time.Time) (service.Provider, error) {
	opened, err := p.sessions.Open(p.served.context.Snapshot(), account)
	if err != nil {
		return nil, err
	}
	limiter, err := p.served.context.Limiter(p.served.pool, account, gmail.Profile{}.BudgetPerSecond(), p.leases)
	if err != nil {
		return nil, err
	}
	return &provider{p: p, account: account, session: opened, limiter: limiter, waitUntil: arrived.Add(p.wait)}, nil
}

// provider is one body request's calls to the account's provider, through its account session.
type provider struct {
	p       *providers
	account string
	session *session.Session
	limiter *lease.Limiter
	// waitUntil is when the request stops waiting for a lease (ADR-0130).
	waitUntil time.Time
}

// Body returns the message's body, under leases in the interactive class, each call leased on its
// own, a second naming the rendered part when the first answers that it is held apart, as
// mail.NextBodyPart decides (ADR-0131).
func (s *provider) Body(ctx context.Context, id string) (mail.MessageBody, error) {
	return session.Call(ctx, s.session, func(ctx context.Context, port mail.Port[context.Context]) (mail.MessageBody, error) {
		var part mail.BodyPart
		for {
			body, err := leased(ctx, s.limiter, port, s.account, mail.OpGetMessageBody, 1, s.waitUntil, s.p.timeout, func(ctx context.Context) (mail.MessageBody, error) {
				return port.GetMessageBody(ctx, id, part)
			})
			next, due := mail.NextBodyPart(part, err)
			if !due {
				return body, err
			}
			part = next
		}
	})
}

// Metadata returns the message's metadata, under a lease in the interactive class, or an error
// wrapping mail.ErrNotFound when the provider no longer holds the message.
func (s *provider) Metadata(ctx context.Context, id string) (mail.MessageMetadata, error) {
	read, err := session.Call(ctx, s.session, func(ctx context.Context, port mail.Port[context.Context]) ([]mail.MessageMetadata, error) {
		return leased(ctx, s.limiter, port, s.account, mail.OpGetMessageMetadata, 1, s.waitUntil, s.p.timeout, func(ctx context.Context) ([]mail.MessageMetadata, error) {
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
// call, since a controller that never hears of a throttle would not slow down. The lease is waited for
// until waitUntil, the request's deadline for its leases, and a wait cut off by it while ctx is still
// live is the API server's failure (ADR-0130). The call then has timeout to answer, and one that has
// not answered by then while ctx is still live is the provider's failure (ADR-0101).
func leased[T any](ctx context.Context, limiter *lease.Limiter, port mail.Port[context.Context], account string, op mail.Operation, messages int,
	waitUntil time.Time, timeout time.Duration, call func(context.Context) (T, error),
) (T, error) {
	var zero T
	cost := port.RateProfile().Cost(mail.ProviderOp{Operation: op, Messages: messages})
	waiting, stopWaiting := context.WithDeadline(ctx, waitUntil)
	_, err := limiter.Acquire(waiting, account, ratecore.Interactive, cost.Weight)
	cutOff := errors.Is(waiting.Err(), context.DeadlineExceeded) && ctx.Err() == nil
	stopWaiting()
	if err != nil && cutOff {
		return zero, fmt.Errorf("leasing the call: no lease by the request's deadline for its leases: %w", err)
	}
	if err != nil {
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
