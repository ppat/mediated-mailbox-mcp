// Package app is backfill's entry package and composition root. It constructs the object graph by hand
// in ordinary code, and backfill/main.go calls it. It sits outside internal, so a composition root
// other than backfill's own can compose it, and every other package of this deployable sits under
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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/accountload/session"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/settings"
)

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// pageSize is how many messages waiting for a scan one page of the second pass reads. Bodies are
// fetched and scanned one at a time, so it bounds a page's rework and not its memory.
const pageSize = 100

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

// Run loads the configuration and the keyring, connects to the database and runs backfill over the
// accounts it reads there, serving the health probe and the metrics endpoint while it runs. It logs
// the effective configuration first, each value with the layer that set it (ADR-0078). The scanner is
// built from its section before anything else starts, so a section it refuses refuses the start. The
// keyring is loaded before any connection is made, so a public key matching none of the private keys
// refuses the start (ADR-0088).
func Run(ctx context.Context, args, environ []string, logger *slog.Logger) error {
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
	connect, err := gmailConnector(registry)
	if err != nil {
		return errors.Join(err, stopProbes())
	}
	work, err := firstPass(pool, scanner, registry, logger)
	if err != nil {
		return errors.Join(err, stopProbes())
	}
	return errors.Join(backfill(ctx, pool, keys, logger, registry, connect, work), stopProbes())
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
	// accounts are the accounts the run serves, sorted, each connected through a provider it has an
	// adapter for.
	accounts []string
	// open opens the session of an account the run serves, which holds the account's token source for
	// the run, the credentials it was built from and their adoption stamp, and the port over it
	// (ADR-0089). A refused credential is read again through it, the one way a running unit of work
	// sees a credential the operator replaced after the snapshot was taken (ADR-0090, ADR-0106).
	open func(account string) (*session.Session, error)
	// policy is the policy of every listed account, loaded once at the start of the run.
	policy policyload.Snapshot
}

// unitOfWork is the work a run does with what it serves, between taking its account snapshot and
// ending. It ends a unit of work over an account's session after each unit it does with it, so a
// rotated refresh token is written back and the latest authentication attempt recorded as soon as the
// unit ends (ADR-0082, ADR-0097).
type unitOfWork func(ctx context.Context, s served) error

// backfill takes the account snapshot once, at the start of the run (ADR-0090), loads the policy of
// every listed account through a loader whose reload-failure series registry serves, and runs work
// with the accounts it can serve, whose sessions hold their token sources for the run and build each
// account's port with connect. A snapshot whose read fails stops the run, since a run that exits holds
// no previous snapshot.
func backfill(ctx context.Context, pool *pgxpool.Pool, keys *open.Keyring, logger *slog.Logger, registry prometheus.Registerer, connect session.Connector, work unitOfWork) error {
	loader := accountload.New(pool, keys, logger, []string{gmailProvider})
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
	holder := session.New(session.Config{
		Loader: loader, DB: pool, Logger: logger, Connectors: map[string]session.Connector{gmailProvider: connect}, Hold: true,
	})
	return work(ctx, served{accounts: holder.Accounts(snapshot), policy: policies.Snapshot(), open: func(account string) (*session.Session, error) {
		return holder.Open(snapshot, account)
	}})
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

// fetch returns the first pass's enumeration over the account's session, under a lease (ADR-0025). A
// refused credential is read again and the call made once more as session.Call says, a new lease
// included.
func fetch(s *session.Session, limiter *lease.Limiter, account string) pass1.Fetch {
	return func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return session.Call(ctx, s, func(ctx context.Context, port mail.Port[context.Context]) (mail.Page[mail.MessageMetadata], error) {
			return pass1.Leased(limiter, port, account)(ctx, token)
		})
	}
}

// body returns the second pass's body fetch over the account's session, under a lease (ADR-0025),
// reading a refused credential again as fetch does.
func body(s *session.Session, limiter *lease.Limiter, account string) pass2.Body {
	return func(ctx context.Context, id string) (mail.MessageBody, error) {
		return session.Call(ctx, s, func(ctx context.Context, port mail.Port[context.Context]) (mail.MessageBody, error) {
			return leasedBody(limiter, port, account)(ctx, id)
		})
	}
}

// firstPass returns the unit of work that runs backfill's two passes over every served account in
// turn (ADR-0017), returning the verdicts another scanner made and the gate skips the gate no longer
// decides as the same skip to pending before the first (ADR-0096, ADR-0098), spending from each
// account's rate budget under the target its state row sets (ADR-0024). Each account's calls go
// through the port its session builds over its token source. The second pass runs for an account once
// its first has ended. Every page of either pass is one unit of work, so the account's session ends a
// unit after each, handing its token over and recording its latest attempt. An account whose pass
// fails leaves the others to run, and the run ends in an error naming it.
func firstPass(pool *pgxpool.Pool, scanner scan.Scanner, registry prometheus.Registerer, logger *slog.Logger) (unitOfWork, error) {
	leaseMetrics, err := lease.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	metrics, err := pass1.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	secondMetrics, err := pass2.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	store, secondStore := pass1.NewPostgres(pool), pass2.NewPostgres(pool)
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	return func(ctx context.Context, s served) error {
		limiter, err := newLimiter(ctx, pool, s.accounts, leaseMetrics)
		if err != nil {
			return err
		}
		var failures []error
		for _, account := range s.accounts {
			sess, err := s.open(account)
			if err != nil {
				return err
			}
			deps := pass1.Deps{
				Store: store, Fetch: fetch(sess, limiter, account),
				Policy: s.policy.For(account), Scanner: scanner, Lookups: lookups,
				RunID: runID, Now: time.Now,
			}
			second := secondDeps(secondStore, body(sess, limiter, account), s.policy.For(account), lookups, scanner)
			err = backfillAccount(ctx, deps, second, account, metrics, secondMetrics, ender(sess), logger)
			if err != nil {
				failures = append(failures, fmt.Errorf("account %s: %w", account, err))
			}
			if ctx.Err() != nil {
				break
			}
		}
		return errors.Join(failures...)
	}, nil
}

// ender returns the hand-over a pass calls after each page, which ends a unit of work over the
// account's session and returns the failures of its hand-over and of its recording joined.
func ender(s *session.Session) func(context.Context, string) error {
	return func(ctx context.Context, _ string) error {
		return errors.Join(s.End(ctx))
	}
}

// secondDeps returns what an account's second pass runs with. The gate decides under the thresholds
// delta sync decides under, and the scanner is the one delta sync builds from the same section, so
// neither workload reopens the other's work (ADR-0096, ADR-0098, ADR-0104).
func secondDeps(store pass2.Store, body pass2.Body, composed policy.Composed, lookups classify.Lookups, scanner scan.Scanner) pass2.Deps {
	return pass2.Deps{
		Store: store, Body: body, Policy: composed, Lookups: lookups, Gate: scangate.DefaultConfig(), Scanner: scanner,
		PageSize: pageSize, RunID: runID, Now: time.Now,
	}
}

// backfillAccount runs backfill over one account. It first returns the verdicts made under another
// scanner and the gate skips the gate no longer decides as the same skip to pending, so they are
// denied whatever the first pass does, then runs the first pass, and the second once the first has
// ended (ADR-0096, ADR-0098, ADR-0017).
func backfillAccount(ctx context.Context, first pass1.Deps, second pass2.Deps, account string, metrics *pass1.Metrics, secondMetrics *pass2.Metrics,
	handOver func(context.Context, string) error, logger *slog.Logger,
) error {
	if err := reopen(ctx, second, account, logger); err != nil {
		return err
	}
	if err := passAccount(ctx, first, account, metrics, handOver, logger); err != nil {
		return err
	}
	return secondPassAccount(ctx, second, account, secondMetrics, handOver, logger)
}

// reopen returns to pending every verdict of the account made under another scanner than the one the
// run scans with, and every stored gate skip the gate under the run's thresholds no longer decides as
// the same skip, before the first pass, so each is denied from the start of the run that sees it
// whatever the first pass does, and reopens the second pass when it returned either or the first pass
// is due again (ADR-0096, ADR-0098).
func reopen(ctx context.Context, deps pass2.Deps, account string, logger *slog.Logger) error {
	r, err := pass2.Reopen(ctx, deps, account)
	if err != nil {
		return err
	}
	if r.Verdicts > 0 {
		logger.Info("verdicts made under another scanner returned to pending", "account", account, "messages", r.Verdicts)
	}
	if r.Skips > 0 {
		logger.Info("gate skips the gate no longer decides as the same skip returned to pending", "account", account, "messages", r.Skips)
	}
	return nil
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

// leasedBody returns the body fetch the second pass uses, a call of port's body read under a lease in
// the batch class (ADR-0025).
func leasedBody(limiter *lease.Limiter, port mail.Port[context.Context], account string) pass2.Body {
	return func(ctx context.Context, id string) (mail.MessageBody, error) {
		return lease.Call(ctx, limiter, port, account, ratecore.Batch, mail.ProviderOp{Operation: mail.OpGetMessageBody}, func(ctx context.Context) (mail.MessageBody, error) {
			return port.GetMessageBody(ctx, id)
		})
	}
}

// secondPassAccount runs the second pass over one account a page at a time, handing the account's
// token over and setting the account's scan backlog after each. It logs counts and identifiers, never
// a body (ADR-0009).
func secondPassAccount(ctx context.Context, deps pass2.Deps, account string, metrics *pass2.Metrics, handOver func(context.Context, string) error, logger *slog.Logger) error {
	p, err := pass2.Open(ctx, deps, account)
	if err != nil {
		return err
	}
	if p.Run() == "" {
		logger.Info("the second pass has ended for the account, or waits for the first, so it is skipped", "account", account)
		return nil
	}
	logger.Info("the second pass runs", "account", account, "run", p.Run(), "page", p.Progress().Checkpoint.Page)
	var handOvers []error
	for {
		step, err := p.Next(ctx)
		if hoErr := handOver(ctx, account); hoErr != nil {
			handOvers = append(handOvers, hoErr)
		}
		if pending, bErr := deps.Store.Backlog(ctx, account); bErr == nil {
			metrics.Backlog(account, pending)
		} else {
			logger.Warn("reading the scan backlog failed", "account", account, "error", bErr)
		}
		if err != nil {
			logger.Error("the second pass failed", "account", account, "run", p.Run(), "error", err)
			return errors.Join(append(handOvers, err)...)
		}
		at := p.Progress()
		if step.Done {
			logger.Info("the second pass ended", "account", account, "run", p.Run(), "pages", at.Counters.Pages,
				"decided", at.Counters.Decided, "scanned", at.Counters.Scanned, "skipped", at.Counters.Skipped, "pending", at.Counters.Pending)
			return errors.Join(handOvers...)
		}
		logger.Info("page made durable", "account", account, "run", p.Run(), "pass", 2, "page", at.Checkpoint.Page, "scanned", at.Counters.Scanned)
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
