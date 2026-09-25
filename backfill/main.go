// Command backfill is backfill, published as mediated-mailbox-backfill.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/policyload"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		slog.Error("backfill stopped", "error", err)
		os.Exit(1)
	}
}

// run builds backfill's object graph for the accounts it is given and loads their policy. The
// database connection is configured by the standard PostgreSQL client environment variables, with
// the password in the mounted file PGPASSFILE names (ADR-0038).
func run(ctx context.Context, accounts []string) error {
	pool, err := pgxpool.New(ctx, "")
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
	slog.Info("policy loaded", "accounts", len(accounts))
	return nil
}
