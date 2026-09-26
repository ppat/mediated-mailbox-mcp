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
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/connection"
	coreconnection "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/settings"
)

// Configuration is backfill's root configuration type, loaded by the configuration library
// (ADR-0078). Its fields are pinned by the test beside this file, so a new value is a visible
// change.
type Configuration struct {
	Database coreconnection.Config `yaml:"database"`
}

// defaults are backfill's defaults. The user is backfill's own runtime role (ADR-0075), and the TLS
// mode is the one that fails closed.
func defaults() Configuration {
	return Configuration{Database: coreconnection.Config{Port: 5432, User: "mediated_mailbox_backfill", SSLMode: "verify-full"}}
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

// run builds backfill's object graph for the accounts it is given and loads their policy. It logs
// the effective configuration first, each value with the layer that set it (ADR-0078).
func run(ctx context.Context, args, environ []string, logger *slog.Logger) error {
	flags, accounts, err := splitArguments(args)
	if err != nil {
		return err
	}
	if err := connection.RefusePasswordVariables(environ); err != nil {
		return err
	}
	loaded, err := settings.Load(defaults(), flags, environ)
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
	if err := coreconnection.Validate(loaded.Config.Database); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	poolConfig, err := connection.PoolConfig(loaded.Config.Database)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	defer pool.Close()
	registry := prometheus.NewRegistry()
	policies, err := policyload.New(pool, accounts, registry)
	if err != nil {
		return err
	}
	if err := policies.Reload(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	logger.Info("policy loaded", "accounts", len(accounts))
	return nil
}

// splitArguments passes an argument starting with a dash to the configuration library as a flag
// and takes any other as an account. An argument holding an equals sign is never an account, so a
// flag written without its dashes is refused rather than taken as one.
func splitArguments(args []string) (flags, accounts []string, err error) {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		case strings.Contains(arg, "="):
			return nil, nil, fmt.Errorf("argument %q: it is neither an account nor a flag, and flags are written --path=VALUE", arg)
		default:
			accounts = append(accounts, arg)
		}
	}
	return flags, accounts, nil
}
