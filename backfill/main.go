// Command backfill is backfill, published as mediated-mailbox-backfill.
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/settings"
)

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// scannerSection is the scanner's section of the configuration, whose revision every verdict and
// masking decision is made under (ADR-0005, ADR-0078).
const scannerSection = "scanner"

// Configuration is backfill's root configuration type, loaded by the configuration library
// (ADR-0078). Its fields are pinned by the test beside this file, so a new value is a visible
// change. No account and no provider credential is configuration. Both come from the database
// (ADR-0080).
type Configuration struct {
	// ProbeListen is the address the health probe and the metrics endpoint listen on, over plain
	// HTTP (ADR-0051).
	ProbeListen string                `yaml:"probe_listen"`
	Database    dbconnectcore.Config  `yaml:"database"`
	Credential  credentialcore.Config `yaml:"credential"`
	Scanner     scan.Config           `yaml:"scanner"`
}

// defaults are backfill's defaults. The user is backfill's own runtime role (ADR-0075), and the TLS
// mode is the one that fails closed. The key files have no default, since a default path assumes
// the environment. The scanner's vocabulary and tuning are the ones the application ships.
func defaults() Configuration {
	return Configuration{
		ProbeListen: ":8080",
		Database:    dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_backfill", SSLMode: "verify-full"},
		Scanner:     scan.DefaultConfig(),
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Environ(), slog.Default())
	stop()
	if err != nil {
		slog.Error("backfill stopped", "error", err)
		os.Exit(1)
	}
}

// run loads the configuration and the keyring, connects to the database and runs backfill over the
// accounts it reads there, serving the health probe and the metrics endpoint while it runs. It logs
// the effective configuration first, each value with the layer that set it (ADR-0078). The scanner is
// built from its section before anything else starts, so a section it refuses refuses the start. The
// keyring is loaded before any connection is made, so a public key matching none of the private keys
// refuses the start (ADR-0088).
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
	if err := credentialcore.Validate(c.Credential); err != nil {
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
	registry := prometheus.NewRegistry()
	ln, err := net.Listen("tcp", c.ProbeListen)
	if err != nil {
		return fmt.Errorf("listening for the probes: %w", err)
	}
	stopProbes := serveProbes(ln, registry, logger)
	work, err := firstPass(pool, scanner, registry, logger)
	if err != nil {
		return errors.Join(err, stopProbes())
	}
	return errors.Join(backfill(ctx, pool, keys, logger, registry, work), stopProbes())
}

// buildScanner builds the scanner from its section of the loaded configuration, under the
// configuration library's revision of that section, so a change to any of its values in any layer
// changes the revision every verdict records, and a change to another section does not (ADR-0005,
// ADR-0078).
func buildScanner(loaded settings.Loaded[Configuration]) (scan.Scanner, error) {
	s, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return scan.Scanner{}, fmt.Errorf("scanner: %w", err)
	}
	return s, nil
}

// serveProbes serves the health probe and the metrics endpoint on ln until the returned function
// stops them (ADR-0051). /healthz answers 200 while the process runs, and /metrics serves the
// process's registry.
func serveProbes(ln net.Listener, registry *prometheus.Registry, logger *slog.Logger) func() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			logger.WarnContext(r.Context(), "writing a probe answer failed", "error", err)
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

// served is what a run serves, taken at its start.
type served struct {
	// sources are the token sources of the accounts the run serves, keyed on the account.
	sources map[string]*gmail.TokenSource
	// policy is the policy of every listed account, loaded once at the start of the run.
	policy policyload.Snapshot
	// handOver hands an account's source's current refresh token over, for the unit of work to call
	// after each unit it does with that source (ADR-0082).
	handOver func(ctx context.Context, account string) error
}

// unitOfWork is the work a run does with what it serves, between taking its account snapshot and
// ending. It calls handOver for an account after each unit of work it does with that account's
// source, so a rotated refresh token is written back as soon as the unit ends (ADR-0082).
type unitOfWork func(ctx context.Context, s served) error

// backfill takes the account snapshot once, at the start of the run (ADR-0090), loads the policy of
// every listed account through a loader whose reload-failure series registry serves, builds a token
// source for each account it can serve and runs work with them. A snapshot whose read fails stops the
// run, since a run that exits holds no previous snapshot.
func backfill(ctx context.Context, pool *pgxpool.Pool, keys *open.Keyring, logger *slog.Logger, registry prometheus.Registerer, work unitOfWork) error {
	loader := accountload.New(pool, keys, logger)
	if err := loader.Load(ctx); err != nil {
		return fmt.Errorf("loading the accounts: %w", err)
	}
	snapshot := loader.Snapshot()
	listed := snapshot.Accounts()
	ids := make([]string, 0, len(listed))
	for _, a := range listed {
		ids = append(ids, a.ID())
	}
	policies, err := policyload.New(pool, ids, registry)
	if err != nil {
		return err
	}
	if err := policies.Reload(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	logger.Info("policy loaded", "accounts", len(ids))
	sources := map[string]*gmail.TokenSource{}
	for account, c := range credentials(snapshot, logger) {
		sources[account] = gmail.NewTokenSource(http.DefaultClient, c)
	}
	return work(ctx, served{sources: sources, policy: policies.Snapshot(), handOver: func(ctx context.Context, account string) error {
		return handOver(ctx, loader, account, sources[account])
	}})
}

// credentials returns the Gmail credentials of each account of the snapshot that backfill can serve,
// keyed on the account. They are the installation's Gmail client and the account's refresh token,
// which is the account's credential as the snapshot opened it (ADR-0083). An account that is not
// connected, whose provider backfill has no adapter for, or whose provider has no client in the
// snapshot is logged and skipped.
func credentials(snapshot *accountload.Snapshot, logger *slog.Logger) map[string]gmail.Credentials {
	out := map[string]gmail.Credentials{}
	for _, a := range snapshot.Accounts() {
		if !a.Connected() {
			logger.Warn("an account is not connected, so it is skipped", "account", a.ID())
			continue
		}
		if a.Provider() != gmailProvider {
			logger.Warn("backfill has no adapter for an account's provider, so it is skipped", "account", a.ID(), "provider", a.Provider())
			continue
		}
		c, ok := snapshot.Client(gmailProvider)
		if !ok {
			logger.Warn("an account's provider has no OAuth client, so it is skipped", "account", a.ID(), "provider", a.Provider())
			continue
		}
		out[a.ID()] = gmail.Credentials{
			ClientID:     c.ID(),
			ClientSecret: string(c.Secret()),
			RefreshToken: string(a.Credential()),
		}
	}
	return out
}

// handOver hands the account's source's current refresh token to the loader at the end of a unit of
// work. The loader writes a rotated one back by compare-and-set and writes an unchanged one nowhere,
// and logs a write-back that fails (ADR-0082, ADR-0089). The credential Persist returns is not handed
// back to the source, which already holds it.
func handOver(ctx context.Context, loader *accountload.Loader, account string, source *gmail.TokenSource) error {
	if source == nil {
		return nil
	}
	_, err := loader.Persist(ctx, account, []byte(source.RefreshToken()))
	return err
}

// firstPass returns the unit of work that runs backfill's first pass over every served account in
// turn (ADR-0017), spending from each account's rate budget under the target its state row sets
// (ADR-0024). Every page is one unit of work, so the account's token is handed over after each. An
// account whose pass fails leaves the others to run, and the run ends in an error naming it.
func firstPass(pool *pgxpool.Pool, scanner scan.Scanner, registry prometheus.Registerer, logger *slog.Logger) (unitOfWork, error) {
	gmailMetrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	leaseMetrics, err := lease.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	metrics, err := pass1.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	store := pass1.NewPostgres(pool)
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	return func(ctx context.Context, s served) error {
		accounts := slices.Sorted(maps.Keys(s.sources))
		limiter, err := newLimiter(ctx, pool, accounts, leaseMetrics)
		if err != nil {
			return err
		}
		var failures []error
		for _, account := range accounts {
			adapter, err := gmail.New(gmail.Config{Account: account, Client: http.DefaultClient, Tokens: s.sources[account], Metrics: gmailMetrics})
			if err != nil {
				return err
			}
			deps := pass1.Deps{
				Store: store, Fetch: pass1.Leased(limiter, adapter, account),
				Policy: s.policy.For(account), Scanner: scanner, Lookups: lookups,
				RunID: runID, Now: time.Now,
			}
			if err := passAccount(ctx, deps, account, metrics, s.handOver, logger); err != nil {
				failures = append(failures, fmt.Errorf("account %s: %w", account, err))
			}
			if ctx.Err() != nil {
				break
			}
		}
		return errors.Join(failures...)
	}, nil
}

// passAccount runs the first pass over one account a page at a time, handing the account's token
// over and counting the page's unclassified senders after each.
func passAccount(ctx context.Context, deps pass1.Deps, account string, metrics *pass1.Metrics, handOver func(context.Context, string) error, logger *slog.Logger) error {
	p, err := pass1.Open(ctx, deps, account)
	if err != nil {
		return err
	}
	if p.Run() == "" {
		logger.Info("the first pass has ended for the account, so it is skipped", "account", account)
		return nil
	}
	logger.Info("the first pass runs", "account", account, "run", p.Run(), "page", p.Progress().Checkpoint.Page)
	var handOvers []error
	for {
		step, err := p.Next(ctx)
		if hoErr := handOver(ctx, account); hoErr != nil {
			handOvers = append(handOvers, hoErr)
		}
		metrics.Count(account, step)
		if err != nil {
			logger.Error("the first pass failed", "account", account, "run", p.Run(), "error", err)
			return errors.Join(append(handOvers, err)...)
		}
		at := p.Progress()
		logger.Info("page made durable", "account", account, "run", p.Run(), "page", at.Checkpoint.Page, "messages", at.Counters.Messages)
		if step.Done {
			logger.Info("the first pass ended", "account", account, "run", p.Run(), "pages", at.Counters.Pages, "messages", at.Counters.Messages)
			return errors.Join(handOvers...)
		}
	}
}

// newLimiter returns the rate limiter for the Gmail accounts, each at the target its state row sets
// (ADR-0024).
func newLimiter(ctx context.Context, pool *pgxpool.Pool, accounts []string, metrics *lease.Metrics) (*lease.Limiter, error) {
	targets, err := loweredTargets(ctx, pool, accounts)
	if err != nil {
		return nil, err
	}
	limiter, err := lease.NewWithTargets(pool, gmail.Profile{}.BudgetPerSecond(), targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
}

// loweredTargets returns the lower target each account's state row sets, a fraction of its
// provider's declared ceiling, leaving out the accounts that set none (ADR-0024).
func loweredTargets(ctx context.Context, db tx.Beginner, accounts []string) (map[string]float64, error) {
	targets := map[string]float64{}
	for _, account := range accounts {
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
	}
	return targets, nil
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
