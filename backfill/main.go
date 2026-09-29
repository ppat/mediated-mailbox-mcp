// Command backfill is backfill, published as mediated-mailbox-backfill.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/settings"
)

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// Configuration is backfill's root configuration type, loaded by the configuration library
// (ADR-0078). Its fields are pinned by the test beside this file, so a new value is a visible
// change. No account and no provider credential is configuration. Both come from the database
// (ADR-0080).
type Configuration struct {
	Database   dbconnectcore.Config  `yaml:"database"`
	Credential credentialcore.Config `yaml:"credential"`
}

// defaults are backfill's defaults. The user is backfill's own runtime role (ADR-0075), and the TLS
// mode is the one that fails closed. The key files have no default, since a default path assumes
// the environment.
func defaults() Configuration {
	return Configuration{Database: dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_backfill", SSLMode: "verify-full"}}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Environ(), slog.Default())
	stop()
	if err != nil {
		slog.Error("backfill stopped", "error", err)
		os.Exit(1)
	}
}

// run loads the configuration and the keyring, connects to the database and runs backfill over the
// accounts it reads there. It logs the effective configuration first, each value with the layer that
// set it (ADR-0078). The keyring is loaded before any connection is made, so a public key matching
// none of the private keys refuses the start (ADR-0088).
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
	if err := dbconnectcore.Validate(loaded.Config.Database); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	if err := credentialcore.Validate(loaded.Config.Credential); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	keys, err := open.Load(loaded.Config.Credential.PublicKeyFile, loaded.Config.Credential.PrivateKeyFiles...)
	if err != nil {
		return fmt.Errorf("loading the keyring: %w", err)
	}
	poolConfig, err := dbconnect.PoolConfig(loaded.Config.Database)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	defer pool.Close()
	// The run does no work between taking its snapshot and handing the tokens over until pass 1 is
	// built into it.
	idle := func(context.Context, map[string]*gmail.TokenSource) error { return nil }
	return backfill(ctx, pool, keys, logger, idle)
}

// unitOfWork is the work a run does with the token sources between taking its account snapshot and
// handing their tokens over.
type unitOfWork func(ctx context.Context, sources map[string]*gmail.TokenSource) error

// backfill takes the account snapshot once, at the start of the run (ADR-0090), loads the policy of
// every listed account, builds a token source for each account it can serve and runs work with
// them. At the end of the run it hands each source's current refresh token to the loader, which
// writes a rotated one back (ADR-0082), whether or not work failed, since a rotation may have
// happened before the failure. The run is backfill's one unit of work until its first pass gives it
// one per page. A snapshot whose read fails stops the run, since a run that exits holds no previous
// snapshot.
func backfill(ctx context.Context, pool *pgxpool.Pool, keys *open.Keyring, logger *slog.Logger, work unitOfWork) error {
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
	registry := prometheus.NewRegistry()
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
	failed := work(ctx, sources)
	return errors.Join(failed, handOver(ctx, loader, sources))
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

// handOver hands each source's current refresh token to the loader at the end of a unit of work.
// The loader writes a rotated one back by compare-and-set and writes an unchanged one nowhere
// (ADR-0082, ADR-0089). Every account is handed over, and the failures are returned together once
// all have been, so the run ends in an error when a rotation did not reach the database. The loader
// logs each failure. The credential Persist returns is not handed back to the source, since the run
// ends after the hand-over.
func handOver(ctx context.Context, loader *accountload.Loader, sources map[string]*gmail.TokenSource) error {
	var failures []error
	for account, source := range sources {
		if _, err := loader.Persist(ctx, account, []byte(source.RefreshToken())); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
