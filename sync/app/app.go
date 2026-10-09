// Package app is delta sync's entry package and composition root. It constructs the object graph by
// hand in ordinary code, and sync/main.go calls it. It sits outside internal, so a composition root
// other than delta sync's own can compose it, and every other package of this deployable sits under
// internal.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/probes"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
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
	// (ADR-0120).
	Scanner scan.Config `yaml:"scanner"`
	// LogLevel is the lowest level the deployable logs at, debug, info, warn or error (ADR-0122).
	LogLevel string `yaml:"log_level"`
}

// defaults are delta sync's defaults. The interval is ADR-0018's, the first window is ADR-0105's,
// and the bound on the waiting messages a tick decides is the top of ADR-0018's steady-state tick.
// The user is delta sync's own runtime role (ADR-0075), and the TLS mode is the one that fails
// closed. The key files have no default, since a default path assumes the environment. The
// scanner's vocabulary and tuning are the ones the application ships.
func defaults() Configuration {
	return Configuration{
		ProbeListen:      ":8080",
		SyncInterval:     5 * time.Minute,
		FirstWindow:      7 * 24 * time.Hour,
		DecisionsPerTick: 200,
		Database:         dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_sync", SSLMode: "verify-full"},
		Scanner:          scan.DefaultConfig(),
		LogLevel:         logging.DefaultLevel,
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

// Run loads the configuration and the keyring, connects to the database and ticks over the accounts
// it reads there until ctx ends, serving the health probe and the metrics endpoint the whole time
// (ADR-0103). It logs the effective configuration first, each value with the layer that set it
// (ADR-0078). The scanner is built from its section before anything else starts, so a section it
// refuses refuses the start. The keyring is loaded before any connection is made, so a public key
// matching none of the private keys refuses the start (ADR-0088).
//
// logger is the logger main.go built over level, at info until Run sets level from log_level once
// the effective configuration is written, so every logger derived from logger follows it (ADR-0122).
func Run(ctx context.Context, args, environ []string, logger *slog.Logger, level *slog.LevelVar) error {
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

// assemble builds everything delta sync serves, which is the process's registry, the Gmail
// adapter's series on it, the syncer with its tick, key-scan, rate-limit and reload-failure series
// on it, and the health probe and the metrics endpoint serving it on ln. Each account's adapter is
// built over client. run and the test of the scrape both call it, so no registry is handed from one
// part of the composition root to another, and every series delta sync emits is the one the metrics
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
	return s, probes.Serve(ln, registry, logger), nil
}

// buildScanner builds the scanner from its section of the loaded configuration, under the
// configuration library's revision of that section, so a change to any of its values in any layer
// changes the revision every verdict and mask records, and a change to another section does not. The
// revision depends on the section's values alone, so delta sync and backfill record the same one for
// the same section (ADR-0005, ADR-0078, ADR-0120).
func buildScanner(loaded settings.Loaded[Configuration]) (scan.Scanner, error) {
	s, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return scan.Scanner{}, fmt.Errorf("scanner: %w", err)
	}
	return s, nil
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
	// source builds an account's token source from its credentials, and ports its Provider Port. The
	// account sessions build every source and port through them.
	source func(gmail.Credentials) *gmail.TokenSource
	ports  ports
	// sessions open each tick's account sessions, a fresh source for each tick, so delta sync holds
	// no source across ticks.
	sessions *session.Holder
	ticks    *tick.Metrics
	keyScan  *reseal.Metrics
	leases   *lease.Metrics
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
	s := &syncer{
		pool: pool, loader: accountload.New(pool, keys, logger, []string{gmailProvider}), registry: registry, scanner: scanner, config: c,
		logger: logger, ports: p, ticks: ticks, keyScan: keyScan, leases: leases,
		source: func(c gmail.Credentials) *gmail.TokenSource { return gmail.NewTokenSource(http.DefaultClient, c) },
	}
	connect := session.Connect(func(c session.Credentials) *gmail.TokenSource {
		return s.source(gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
	}, func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return s.ports(account, source)
	})
	s.sessions = session.New(session.Config{Loader: s.loader, DB: pool, Logger: logger, Connectors: map[string]session.Connector{gmailProvider: connect}})
	return s, nil
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
	accounts := s.sessions.Accounts(snapshot)
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
		if err := s.tickAccount(ctx, snapshot, account, limiter, policy); err != nil {
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

// tickAccount runs one tick over the account in a session opened with the credentials snapshot holds
// for it, over a token source built for the tick, then ends the session's unit of work, which hands
// its token over with the adoption stamp of the credential it holds and records its latest
// authentication attempt, a tick that failed included (ADR-0089, ADR-0097). It logs counts and
// identifiers, never a body (ADR-0009).
func (s *syncer) tickAccount(ctx context.Context, snapshot *accountload.Snapshot, account string, limiter *lease.Limiter, policy policyload.Snapshot) error {
	sess, err := s.sessions.Open(snapshot, account)
	if err != nil {
		return err
	}
	provider := &reauthorizing{session: sess, limiter: limiter, account: account}
	result, err := tick.Run(ctx, s.tickDeps(account, provider, policy), account)
	s.ticks.Count(account, result)
	c := result.Progress.Counters
	if err != nil {
		s.logger.Error("the account's tick failed", "account", account, "run", result.Run, "recovery", result.Recovery, "error", err)
	} else {
		s.logger.Info("the account's tick ended", "account", account, "run", result.Run, "recovery", result.Recovery, "gap", result.Gap,
			"added", c.Added, "modified", c.Modified, "removed", c.Removed, "decided", c.Decided, "scanned", c.Scanned, "skipped", c.Skipped)
	}
	handedOver, recorded := sess.End(ctx)
	return errors.Join(err, handedOver, recorded)
}

// tickDeps returns what the account's tick runs with over provider. The gate decides under the
// thresholds backfill decides under and the scanner is built from the section backfill reads, so
// neither workload reopens the other's work (ADR-0120, ADR-0098, ADR-0104).
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
// port over the account's session, which reads a credential the provider refuses again from the
// account's row before the refusal is returned. A row holding another credential, one the operator
// stored since the tick took its snapshot, has the source and the port built again over it with the
// client the account now connects through (ADR-0106), kept for the tick's later calls and its
// hand-over, and the call made once more, so a re-authorization reaches the tick at the call it
// refused (ADR-0090). A row holding the refused credential, or none, returns the refusal.
type reauthorizing struct {
	session *session.Session
	limiter *lease.Limiter
	account string
}

var _ tick.Provider = (*reauthorizing)(nil)

// leased makes call over the account's leased port through the session. The call made again goes
// through call whole, a new lease included.
func leased[T any](ctx context.Context, r *reauthorizing, call func(context.Context, tick.Leased) (T, error)) (T, error) {
	return session.Call(ctx, r.session, func(ctx context.Context, port mail.Port[context.Context]) (T, error) {
		return call(ctx, tick.Leased{Limiter: r.limiter, Port: port, Account: r.account})
	})
}

// CurrentCursor implements tick.Provider.
func (r *reauthorizing) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Cursor, error) { return l.CurrentCursor(ctx) })
}

// ChangesSince implements tick.Provider.
func (r *reauthorizing) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.ChangeSet, error) { return l.ChangesSince(ctx, c) })
}

// GetMessageMetadata implements tick.Provider.
func (r *reauthorizing) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) ([]mail.MessageMetadata, error) {
		return l.GetMessageMetadata(ctx, ids)
	})
}

// ListThreads implements tick.Provider.
func (r *reauthorizing) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Page[mail.ThreadMetadata], error) {
		return l.ListThreads(ctx, q, page)
	})
}

// GetMessageBody implements tick.Provider.
func (r *reauthorizing) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.MessageBody, error) { return l.GetMessageBody(ctx, id) })
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
