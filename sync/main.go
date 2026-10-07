// Command sync is delta sync, published as mediated-mailbox-sync.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/authentication"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/logging"
	logcore "github.com/ppat/mediated-mailbox-mcp/logging/core"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/tick"
)

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// pageSize is how many waiting messages one read of a tick's scanning returns. Bodies are fetched and
// scanned one at a time, so it bounds a read's rework and not its memory.
const pageSize = 100

// scannerSection is the scanner's section of the configuration, whose revision every verdict and
// masking decision is made under (ADR-0005, ADR-0078).
const scannerSection = "scanner"

// Configuration is delta sync's root configuration type, loaded by the configuration library
// (ADR-0078). Its fields are pinned by the test beside this file, so a new value is a visible change.
// No account and no provider credential is configuration. Both come from the database (ADR-0080).
type Configuration struct {
	// ProbeListen is the address the health probe and the metrics endpoint listen on, over plain
	// HTTP (ADR-0051).
	ProbeListen string `yaml:"probe_listen"`
	// SyncInterval is how often a tick starts, the sync interval (ADR-0018, ADR-0103).
	SyncInterval time.Duration `yaml:"sync_interval"`
	// FirstWindow is how far back an account with no cursor is reconciled at its first tick
	// (ADR-0105).
	FirstWindow time.Duration `yaml:"first_window"`
	// DecisionsPerTick bounds how many waiting messages one tick decides for an account (ADR-0104).
	DecisionsPerTick int                   `yaml:"decisions_per_tick"`
	Database         dbconnectcore.Config  `yaml:"database"`
	Credential       credentialcore.Config `yaml:"credential"`
	// Scanner is the scanner's section, which must be backfill's, or each reopens the other's work
	// (ADR-0096).
	Scanner scan.Config `yaml:"scanner"`
	// Log is the log section, which sets the level the deployable logs at (ADR-0119).
	Log logcore.Config `yaml:"log"`
}

// defaults are delta sync's defaults. The interval is ADR-0018's five minutes, the first window seven
// days, and a tick decides up to two hundred waiting messages, the top of ADR-0018's steady-state
// tick. The user is delta sync's own runtime role (ADR-0075), and the TLS mode is the one that fails
// closed. The key files have no default, since a default path assumes the environment. The scanner's
// vocabulary and tuning are the ones the application ships, and the log level is info (ADR-0119).
func defaults() Configuration {
	return Configuration{
		ProbeListen:      ":8080",
		SyncInterval:     5 * time.Minute,
		FirstWindow:      7 * 24 * time.Hour,
		DecisionsPerTick: 200,
		Database:         dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_sync", SSLMode: "verify-full"},
		Scanner:          scan.DefaultConfig(),
		Log:              logcore.Default(),
	}
}

// validate refuses a configuration delta sync cannot tick with. The interval, the first window and
// the bound on a tick's decisions are positive.
func validate(c Configuration) error {
	switch {
	case c.SyncInterval <= 0:
		return fmt.Errorf("sync_interval %s is not positive", c.SyncInterval)
	case c.FirstWindow <= 0:
		return fmt.Errorf("first_window %s is not positive", c.FirstWindow)
	case c.DecisionsPerTick <= 0:
		return fmt.Errorf("decisions_per_tick %d is not positive", c.DecisionsPerTick)
	}
	if err := dbconnectcore.Validate(c.Database); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

// main logs a start refused before the configuration is loaded through the initial logger, and
// anything later through the logger run builds, which it also sets as the process default for the
// code the project does not own (ADR-0119).
func main() {
	logger := logging.Initial(os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Environ(), os.Stdout, func(configured *slog.Logger) {
		logger = configured
		slog.SetDefault(configured)
	})
	stop()
	if err != nil {
		logger.Error("delta sync stopped", "error", err)
		os.Exit(1)
	}
}

// run loads the configuration and the keyring, connects to the database and ticks over the accounts
// it reads there until ctx ends, serving the health probe and the metrics endpoint the whole time
// (ADR-0103). It builds the logger the log section configures, writing to out, hands it to adopt and
// to every part that logs, and logs the effective configuration through it first, each value with
// the layer that set it (ADR-0078, ADR-0119). The scanner is built from its section before anything else starts, so a section it
// refuses refuses the start. The keyring is loaded before any connection is made, so a public key
// matching none of the private keys refuses the start (ADR-0088).
func run(ctx context.Context, args, environ []string, out io.Writer, adopt func(*slog.Logger)) error {
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
	logger, err := logging.New(out, loaded.Config.Log, loaded.Values)
	if err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	adopt(logger)
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	c := loaded.Config
	if err := validate(c); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
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
	ln, err := net.Listen("tcp", c.ProbeListen)
	if err != nil {
		return fmt.Errorf("listening for the probes: %w", err)
	}
	s, stopProbes, err := assemble(ln, pool, keys, scanner, c, http.DefaultClient, logger)
	if err != nil {
		return errors.Join(err, ln.Close())
	}
	return errors.Join(s.every(ctx, c.SyncInterval), stopProbes())
}

// assemble builds everything delta sync serves: the process's registry, the Gmail adapter's series
// on it, the syncer with its tick, key-scan, rate-limit and reload-failure series on it, and the
// health probe and the metrics endpoint serving it on ln. Each account's adapter is built over
// client. run and the test of the scrape both call it, so no registry is handed from one part of
// the composition root to another, and every series delta sync emits is the one the metrics
// endpoint serves (ADR-0077, ADR-0103). It returns the syncer and the function that stops the
// probes.
func assemble(ln net.Listener, pool *pgxpool.Pool, keys *open.Keyring, scanner scan.Scanner, c Configuration, client *http.Client, logger *slog.Logger) (*syncer, func() error, error) {
	registry := prometheus.NewRegistry()
	gmailMetrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return nil, nil, err
	}
	s, err := newSyncer(pool, keys, scanner, c, registry, logger, gmailPorts(gmailMetrics, client))
	if err != nil {
		return nil, nil, err
	}
	return s, serveProbes(ln, registry, logger), nil
}

// buildScanner builds the scanner from its section of the loaded configuration, under the
// configuration library's revision of that section, so a change to any of its values in any layer
// changes the revision every verdict and mask records, and a change to another section does not. The
// revision depends on the section's values alone, so delta sync and backfill record the same one for
// the same section (ADR-0005, ADR-0078, ADR-0096).
func buildScanner(loaded settings.Loaded[Configuration]) (scan.Scanner, error) {
	s, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return scan.Scanner{}, fmt.Errorf("scanner: %w", err)
	}
	return s, nil
}

// serveProbes serves the health probe and the metrics endpoint on ln until the returned function
// stops them (ADR-0051). /healthz answers 200 while the process runs, and /metrics serves the
// process's registry, between ticks as during them (ADR-0103).
func serveProbes(ln net.Listener, registry *prometheus.Registry, logger *slog.Logger) func() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
		}
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		served <- err
	}()
	logger.Info("serving the probes", "listen", ln.Addr().String())
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(srv.Shutdown(ctx), <-served)
	}
}

// ports builds the Provider Port an account's tick calls, from the account's token source.
type ports func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error)

// gmailPorts builds a Gmail adapter over client for each account, every one counting its requests on
// the process's adapter series (ADR-0077).
func gmailPorts(metrics *gmail.Metrics, client *http.Client) ports {
	return func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: metrics})
	}
}

// syncer is delta sync between and during its ticks. It holds the account snapshot's loader, the
// policy loader and the process's series, which every tick shares, so each series keeps its value
// between ticks (ADR-0103).
type syncer struct {
	pool     *pgxpool.Pool
	loader   *accountload.Loader
	policies *policyload.Loader
	registry prometheus.Registerer
	scanner  scan.Scanner
	config   Configuration
	logger   *slog.Logger
	// source builds an account's token source from its credentials, and ports its Provider Port.
	source  func(gmail.Credentials) *gmail.TokenSource
	ports   ports
	ticks   *tick.Metrics
	keyScan *reseal.Metrics
	leases  *lease.Metrics
	// served are the accounts the last tick served, whose series a later tick that no longer serves
	// them removes.
	served []string
}

// newSyncer returns delta sync with its series registered on registry.
func newSyncer(pool *pgxpool.Pool, keys *open.Keyring, scanner scan.Scanner, c Configuration, registry prometheus.Registerer, logger *slog.Logger, p ports) (*syncer, error) {
	ticks, err := tick.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	keyScan, err := reseal.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	leases, err := lease.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	return &syncer{
		pool: pool, loader: accountload.New(pool, keys, logger, []string{gmailProvider}), registry: registry, scanner: scanner, config: c,
		logger: logger, ports: p, ticks: ticks, keyScan: keyScan, leases: leases,
		source: func(c gmail.Credentials) *gmail.TokenSource { return gmail.NewTokenSource(http.DefaultClient, c) },
	}, nil
}

// every ticks at once and then on every tick of interval until ctx ends. A tick that fails is logged,
// and the next one starts on time. A tick that outlasts the interval delays the next one rather than
// overlapping it (ADR-0103).
func (s *syncer) every(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.tick(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("the tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// tick takes the account snapshot, re-seals what it opened with a key that is not the current one
// and sets the scan series, loads the policy of every listed account, and ticks each account it can
// serve in turn (ADR-0090, ADR-0092, ADR-0041). A snapshot whose read fails keeps the previous one,
// which the loader logs. Each account's tick is a unit of work, after which its token is handed over
// and its latest authentication attempt recorded (ADR-0082, ADR-0097). An account whose tick fails
// leaves the others to tick, and the tick ends in an error naming it.
func (s *syncer) tick(ctx context.Context) error {
	if err := s.loader.Load(ctx); err != nil && ctx.Err() != nil {
		return err
	}
	s.loader.Reseal(ctx)
	s.keyScan.Set(s.loader.ResealClients(ctx, reseal.Writer(s.pool)))
	snapshot := s.loader.Snapshot()
	var ids []string
	for _, a := range snapshot.Accounts() {
		ids = append(ids, a.ID())
	}
	policy, err := s.loadPolicy(ctx, ids)
	if err != nil {
		s.logger.Error("the policy was not reloaded, so the policy loaded before stays", "error", err)
	}
	sources, err := credentials(snapshot, s.logger)
	if err != nil {
		return err
	}
	accounts := slices.Sorted(maps.Keys(sources))
	for _, gone := range s.served {
		if !slices.Contains(accounts, gone) {
			s.ticks.Forget(gone)
		}
	}
	s.served = accounts
	if len(accounts) == 0 {
		return nil
	}
	limiter, err := newLimiter(ctx, s.pool, accounts, s.leases)
	if err != nil {
		return err
	}
	var failures []error
	for _, account := range accounts {
		held, _ := snapshot.Account(account)
		if err := s.tickAccount(ctx, account, sources[account], held.Adoption(), limiter, policy); err != nil {
			failures = append(failures, fmt.Errorf("account %s: %w", account, err))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

// loadPolicy loads the policy of accounts through the shared policy loader, building it at the first
// tick that lists an account, so a tick decides against the policy as it stands, which the delisting
// comparison needs (ADR-0037, ADR-0041). A load that fails keeps the loader's active snapshot.
func (s *syncer) loadPolicy(ctx context.Context, accounts []string) (policyload.Snapshot, error) {
	if len(accounts) == 0 {
		return policyload.Snapshot{}, nil
	}
	if s.policies == nil {
		built, err := policyload.New(s.pool, accounts, s.registry)
		if err != nil {
			return policyload.Snapshot{}, err
		}
		s.policies = built
	} else if err := s.policies.SetAccounts(accounts); err != nil {
		return s.policies.Snapshot(), err
	}
	err := s.policies.Reload(ctx)
	return s.policies.Snapshot(), err
}

// tickAccount runs one tick over the account with a token source built from its credentials, then
// hands its token over with the adoption stamp of the credential it holds and records its latest
// authentication attempt, a tick that failed included. adoption is the stamp of the credential the
// snapshot held when the tick started (ADR-0089).
// It logs counts and identifiers, never a body (ADR-0009).
func (s *syncer) tickAccount(ctx context.Context, account string, creds gmail.Credentials, adoption uint64, limiter *lease.Limiter, policy policyload.Snapshot) error {
	source := s.source(creds)
	port, err := s.ports(account, source)
	if err != nil {
		return err
	}
	provider := &reauthorizing{syncer: s, account: account, creds: creds, source: source, adoption: adoption, leased: tick.Leased{Limiter: limiter, Port: port, Account: account}}
	result, err := tick.Run(ctx, s.tickDeps(account, provider, policy), account)
	s.ticks.Count(account, result)
	c := result.Progress.Counters
	if err != nil {
		s.logger.Error("the account's tick failed", "account", account, "run", result.Run, "recovery", result.Recovery, "error", err)
	} else {
		s.logger.Info("the account's tick ended", "account", account, "run", result.Run, "recovery", result.Recovery, "gap", result.Gap,
			"added", c.Added, "modified", c.Modified, "removed", c.Removed, "decided", c.Decided, "scanned", c.Scanned, "skipped", c.Skipped)
	}
	return errors.Join(err, handOver(ctx, s.loader, account, provider.adoption, provider.source), recordAttempt(ctx, s.pool, account, provider.source.LastAttempt()))
}

// tickDeps returns what the account's tick runs with over provider. The gate decides under the
// thresholds backfill decides under and the scanner is built from the section backfill reads, so
// neither workload reopens the other's work (ADR-0096, ADR-0098, ADR-0104).
func (s *syncer) tickDeps(account string, provider tick.Provider, policy policyload.Snapshot) tick.Deps {
	return tick.Deps{
		Store:       tick.NewPostgres(s.pool),
		Provider:    provider,
		Policy:      policy.For(account),
		Lookups:     classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		Gate:        scangate.DefaultConfig(),
		Scanner:     s.scanner,
		FirstWindow: s.config.FirstWindow,
		Decisions:   s.config.DecisionsPerTick,
		PageSize:    pageSize,
		RunID:       runID,
		Now:         time.Now,
	}
}

// reauthorizing is the Provider a tick calls for one account. Each call goes through the leased
// port over the account's token source, and a call the provider refuses as a refused credential reads
// the credential again from the account's row before the refusal is returned. A row holding another
// credential, one the operator stored since the tick took its snapshot, has the source and the port
// built again over it with the client the account now connects through (ADR-0106), kept for the tick's later calls and its hand-over, and the call made once more,
// so a re-authorization reaches the tick at the call it refused (ADR-0090). A row holding the refused
// credential, or none, returns the refusal.
type reauthorizing struct {
	syncer  *syncer
	account string
	creds   gmail.Credentials
	source  *gmail.TokenSource
	// adoption is the adoption stamp of the credential source was built over, which the hand-over
	// names (ADR-0089).
	adoption uint64
	leased   tick.Leased
}

var _ tick.Provider = (*reauthorizing)(nil)

// retried makes call over the account's leased port, reading a refused credential again as
// reauthorizing describes. The call made again goes through call whole, a new lease included.
func retried[T any](ctx context.Context, r *reauthorizing, call func(context.Context, tick.Leased) (T, error)) (T, error) {
	out, err := call(ctx, r.leased)
	if !errors.Is(err, mail.ErrAuthentication) {
		return out, err
	}
	reread, rerr := r.syncer.loader.Reread(ctx, r.account)
	if rerr != nil {
		return out, errors.Join(err, fmt.Errorf("reading the refused credential again: %w", rerr))
	}
	held := r.creds
	held.RefreshToken = r.source.RefreshToken()
	creds, ok := sourceCredentials(reread)
	if !ok || creds == held {
		return out, err
	}
	source := r.syncer.source(creds)
	port, cerr := r.syncer.ports(r.account, source)
	if cerr != nil {
		return out, errors.Join(err, cerr)
	}
	r.creds, r.source, r.adoption, r.leased.Port = creds, source, reread.Adoption(), port
	return call(ctx, r.leased)
}

// CurrentCursor implements tick.Provider.
func (r *reauthorizing) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return retried(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Cursor, error) { return l.CurrentCursor(ctx) })
}

// ChangesSince implements tick.Provider.
func (r *reauthorizing) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	return retried(ctx, r, func(ctx context.Context, l tick.Leased) (mail.ChangeSet, error) { return l.ChangesSince(ctx, c) })
}

// GetMessageMetadata implements tick.Provider.
func (r *reauthorizing) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	return retried(ctx, r, func(ctx context.Context, l tick.Leased) ([]mail.MessageMetadata, error) {
		return l.GetMessageMetadata(ctx, ids)
	})
}

// ListThreads implements tick.Provider.
func (r *reauthorizing) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	return retried(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Page[mail.ThreadMetadata], error) {
		return l.ListThreads(ctx, q, page)
	})
}

// GetMessageBody implements tick.Provider.
func (r *reauthorizing) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	return retried(ctx, r, func(ctx context.Context, l tick.Leased) (mail.MessageBody, error) { return l.GetMessageBody(ctx, id) })
}

// credentials returns the Gmail credentials of each account of the snapshot that delta sync can
// serve, keyed on the account. They are the OAuth client the account connects through and the
// account's refresh token, which is the account's credential as the snapshot opened it (ADR-0106).
// An account that is not connected, or whose provider delta sync has no adapter for, is logged and
// skipped. The snapshot connects no Gmail account without its client, since the loader is told Gmail
// authenticates through one, so a connected Gmail account without a client is an error.
func credentials(snapshot *accountload.Snapshot, logger *slog.Logger) (map[string]gmail.Credentials, error) {
	out := map[string]gmail.Credentials{}
	for _, a := range snapshot.Accounts() {
		if !a.Connected() {
			logger.Warn("an account is not connected, so it is skipped", "account", a.ID())
			continue
		}
		if a.Provider() != gmailProvider {
			logger.Warn("delta sync has no adapter for an account's provider, so it is skipped", "account", a.ID(), "provider", a.Provider())
			continue
		}
		c, ok := sourceCredentials(a)
		if !ok {
			return nil, fmt.Errorf("account %s: connected without the OAuth client its provider authenticates through", a.ID())
		}
		out[a.ID()] = c
	}
	return out, nil
}

// sourceCredentials returns the Gmail credentials an account's token source is built from, the OAuth
// client the account connects through and the account's credential, as the loader paired them, and
// false when the account is not connected through a client. Every token source this root builds,
// from a snapshot or from a credential read again after a refusal, takes its credentials here
// (ADR-0106).
func sourceCredentials(a accountload.Account) (gmail.Credentials, bool) {
	c, ok := a.Client()
	if !a.Connected() || !ok {
		return gmail.Credentials{}, false
	}
	return gmail.Credentials{ClientID: c.ID(), ClientSecret: string(c.Secret()), RefreshToken: string(a.Credential())}, true
}

// handOver hands the account's source's current refresh token to the loader at the end of a unit of
// work, with the adoption stamp of the credential the source was built over. The loader discards it
// when the stamp has moved since, because the loader adopted a value someone else stored that the
// source does not hold, and otherwise writes a rotated one back by compare-and-set and an unchanged
// one nowhere, and logs a write-back that fails (ADR-0082, ADR-0089).
func handOver(ctx context.Context, loader *accountload.Loader, account string, adoption uint64, source *gmail.TokenSource) error {
	_, err := loader.HandOver(ctx, account, adoption, []byte(source.RefreshToken()))
	return err
}

// recordAttempt records the latest authentication attempt the account's source reported on the
// account's state row at the end of a unit of work. The zero attempt, from a source that has made
// none, records nothing, and the statement keeps a later attempt another deployable recorded
// (ADR-0097).
func recordAttempt(ctx context.Context, db tx.Beginner, account string, attempt mail.AuthAttempt) error {
	if attempt == (mail.AuthAttempt{}) {
		return nil
	}
	err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
		return authentication.New(t).RecordAuthentication(ctx, authentication.RecordAuthenticationParams{
			AttemptedAt: pgtype.Timestamptz{Time: time.UnixMilli(int64(attempt.At)).UTC(), Valid: true},
			Outcome:     pgtype.Text{String: string(attempt.Outcome), Valid: true},
			AccountID:   account,
		})
	})
	if err != nil {
		return fmt.Errorf("account %s: recording the last authentication attempt: %w", account, err)
	}
	return nil
}

// newLimiter returns the rate limiter for the Gmail accounts, each at the target its state row sets
// (ADR-0024).
func newLimiter(ctx context.Context, pool *pgxpool.Pool, accounts []string, metrics *lease.Metrics) (*lease.Limiter, error) {
	targets := map[string]float64{}
	for _, account := range accounts {
		err := tx.Run(ctx, pool, account, func(t pgx.Tx) error {
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
	}
	limiter, err := lease.NewWithTargets(pool, gmail.Profile{}.BudgetPerSecond(), targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
