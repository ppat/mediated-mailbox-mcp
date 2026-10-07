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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
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
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/authentication"
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
	connect, err := gmailPorts(registry)
	if err != nil {
		return errors.Join(err, stopProbes())
	}
	work, err := firstPass(pool, scanner, registry, logger, connect)
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

// holding is what a run holds for one account it serves: the token source the account's calls go
// through, the credentials it was built from, which are the account's client and the credential the
// source started with, and the adoption stamp of that credential, which the hand-over names
// (ADR-0089). They are replaced together, so a source never hands over another's stamp.
type holding struct {
	source   *gmail.TokenSource
	built    gmail.Credentials
	adoption uint64
}

// served is what a run serves, taken at its start.
type served struct {
	// sources hold the token source of each account the run serves with its adoption stamp, keyed on
	// the account.
	sources map[string]holding
	// policy is the policy of every listed account, loaded once at the start of the run.
	policy policyload.Snapshot
	// handOver hands over what an account's source holds, its current refresh token and its latest
	// authentication attempt, for the unit of work to call after each unit it does with that source
	// (ADR-0082, ADR-0097).
	handOver func(ctx context.Context, account string) error
	// reauthorize reads the account's credential again from its state row, with the client the
	// account now connects through, after the provider refused the one its source holds (ADR-0090,
	// ADR-0106). When the row holds another credential or the account names another client, it makes
	// a source from that pair, with the re-read's adoption stamp, the account's holding, so handOver
	// reads both from then on, and returns the source. It returns nil when the pair is the one the
	// source holds, which is the one refused, or the account is no longer connected, so the refusal
	// stands.
	reauthorize func(ctx context.Context, account string) (*gmail.TokenSource, error)
}

// unitOfWork is the work a run does with what it serves, between taking its account snapshot and
// ending. It calls handOver for an account after each unit of work it does with that account's
// source, so a rotated refresh token is written back and the latest authentication attempt recorded
// as soon as the unit ends (ADR-0082, ADR-0097).
type unitOfWork func(ctx context.Context, s served) error

// backfill takes the account snapshot once, at the start of the run (ADR-0090), ends at once when it
// lists no account, loads the policy of every listed account through a loader whose reload-failure series registry serves, builds a token
// source for each account it can serve and runs work with them. A snapshot whose read fails stops the
// run, since a run that exits holds no previous snapshot. A credential the provider refuses is read
// again from its row through the loader, the one way a running unit of work sees a credential the
// operator replaced after the snapshot was taken (ADR-0090).
func backfill(ctx context.Context, pool *pgxpool.Pool, keys *open.Keyring, logger *slog.Logger, registry prometheus.Registerer, work unitOfWork) error {
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
	if len(ids) == 0 {
		// Before an account is connected a run has nothing to serve, as when the deployment runs it
		// at install, and it ends as having done its work, as delta sync and the mediator serve no
		// account until one is listed.
		logger.Info("no account is listed, so the run has nothing to do")
		return nil
	}
	policies, err := policyload.New(pool, ids, registry)
	if err != nil {
		return err
	}
	if err := policies.Reload(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	logger.Info("policy loaded", "accounts", len(ids))
	sources := map[string]holding{}
	creds, err := credentials(snapshot, logger)
	if err != nil {
		return err
	}
	for account, c := range creds {
		a, _ := snapshot.Account(account)
		sources[account] = holding{source: gmail.NewTokenSource(http.DefaultClient, c), built: c, adoption: a.Adoption()}
	}
	return work(ctx, served{sources: sources, policy: policies.Snapshot(), handOver: func(ctx context.Context, account string) error {
		held := sources[account]
		var attempt mail.AuthAttempt
		if held.source != nil {
			attempt = held.source.LastAttempt()
		}
		return errors.Join(handOver(ctx, loader, account, held), recordAttempt(ctx, pool, account, attempt))
	}, reauthorize: func(ctx context.Context, account string) (*gmail.TokenSource, error) {
		reread, err := loader.Reread(ctx, account)
		if err != nil {
			return nil, err
		}
		held := sources[account].built
		held.RefreshToken = sources[account].source.RefreshToken()
		c, ok := sourceCredentials(reread)
		if !ok || c == held {
			return nil, nil
		}
		sources[account] = holding{source: gmail.NewTokenSource(http.DefaultClient, c), built: c, adoption: reread.Adoption()}
		return sources[account].source, nil
	}})
}

// credentials returns the Gmail credentials of each account of the snapshot that backfill can serve,
// keyed on the account. They are the OAuth client the account connects through and the account's
// refresh token, which is the account's credential as the snapshot opened it (ADR-0106). An account
// that is not connected, or whose provider backfill has no adapter for, is logged and skipped. The
// snapshot connects no Gmail account without its client, since the loader is told Gmail
// authenticates through one, so a connected Gmail account without a client is an error.
func credentials(snapshot *accountload.Snapshot, logger *slog.Logger) (map[string]gmail.Credentials, error) {
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
func handOver(ctx context.Context, loader *accountload.Loader, account string, held holding) error {
	if held.source == nil {
		return nil
	}
	_, err := loader.HandOver(ctx, account, held.adoption, []byte(held.source.RefreshToken()))
	return err
}

// recordAttempt records the latest authentication attempt the account's source reported on the
// account's state row at the end of a unit of work. The zero attempt, from a source that has made
// none, records nothing, and the statement keeps a later attempt another deployable recorded
// (ADR-0097). A recording that fails is returned, and the next unit of work records the attempt
// again.
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

// connector builds the port an account's calls go through, over the token source holding the
// account's credential.
type connector func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error)

// gmailPorts returns the connector that builds a Gmail adapter over the source, counting its requests
// on series registry serves, which every adapter it builds shares.
func gmailPorts(registry prometheus.Registerer) (connector, error) {
	metrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	return func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: http.DefaultClient, Tokens: source, Metrics: metrics})
	}, nil
}

// connection is one account's port, built by connect over the account's token source, and built
// again over a new source when the provider refuses the credential the port's source holds and the
// account's row holds another (ADR-0090).
type connection struct {
	account     string
	port        mail.Port[context.Context]
	connect     connector
	reauthorize func(ctx context.Context, account string) (*gmail.TokenSource, error)
}

// retried makes call over the account's port. When the provider refuses the credential, the
// credential is read again from the account's row before the refusal is returned. A row holding
// another credential has the port built again over a source holding it with the client the account
// now connects through, kept for the account's later calls, and the call made once more over it, so a
// re-authorization reaches the run at the call it refused rather than the next run (ADR-0090,
// ADR-0106). The call made again goes through call whole, a new
// lease included. A row holding the refused credential, or none, returns the refusal.
func retried[T any](ctx context.Context, c *connection, call func(context.Context, mail.Port[context.Context]) (T, error)) (T, error) {
	out, err := call(ctx, c.port)
	if !errors.Is(err, mail.ErrAuthentication) {
		return out, err
	}
	source, rerr := c.reauthorize(ctx, c.account)
	if rerr != nil {
		return out, errors.Join(err, fmt.Errorf("reading the refused credential again: %w", rerr))
	}
	if source == nil {
		return out, err
	}
	port, cerr := c.connect(c.account, source)
	if cerr != nil {
		return out, errors.Join(err, cerr)
	}
	c.port = port
	return call(ctx, port)
}

// fetch returns the first pass's enumeration over the account's port, under a lease (ADR-0025).
func (c *connection) fetch(limiter *lease.Limiter) pass1.Fetch {
	return func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return retried(ctx, c, func(ctx context.Context, port mail.Port[context.Context]) (mail.Page[mail.MessageMetadata], error) {
			return pass1.Leased(limiter, port, c.account)(ctx, token)
		})
	}
}

// body returns the second pass's body fetch over the account's port, under a lease (ADR-0025).
func (c *connection) body(limiter *lease.Limiter) pass2.Body {
	return func(ctx context.Context, id string) (mail.MessageBody, error) {
		return retried(ctx, c, func(ctx context.Context, port mail.Port[context.Context]) (mail.MessageBody, error) {
			return leasedBody(limiter, port, c.account)(ctx, id)
		})
	}
}

// firstPass returns the unit of work that runs backfill's two passes over every served account in
// turn (ADR-0017), returning the verdicts another scanner made and the gate skips the gate no longer
// decides as the same skip to pending before the first (ADR-0096, ADR-0098), spending from each
// account's rate budget under the target its state row sets (ADR-0024). Each account's calls go
// through the port connect builds over its token source. The second pass runs for an account once its
// first has ended. Every page of either pass is one unit of work, so the account's token is handed
// over after each. An account whose pass fails leaves the others to run, and the run ends in an error
// naming it.
func firstPass(pool *pgxpool.Pool, scanner scan.Scanner, registry prometheus.Registerer, logger *slog.Logger, connect connector) (unitOfWork, error) {
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
		accounts := slices.Sorted(maps.Keys(s.sources))
		limiter, err := newLimiter(ctx, pool, accounts, leaseMetrics)
		if err != nil {
			return err
		}
		var failures []error
		for _, account := range accounts {
			port, err := connect(account, s.sources[account].source)
			if err != nil {
				return err
			}
			c := &connection{account: account, port: port, connect: connect, reauthorize: s.reauthorize}
			deps := pass1.Deps{
				Store: store, Fetch: c.fetch(limiter),
				Policy: s.policy.For(account), Scanner: scanner, Lookups: lookups,
				RunID: runID, Now: time.Now,
			}
			second := secondDeps(secondStore, c.body(limiter), s.policy.For(account), lookups, scanner)
			err = backfillAccount(ctx, deps, second, account, metrics, secondMetrics, s.handOver, logger)
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
