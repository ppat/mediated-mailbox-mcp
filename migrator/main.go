// Command migrator is the migrator, published as mediated-mailbox-migrator, which the migration step
// runs (ADR-0048, ADR-0139). It applies the migration chain its binary carries and exits.
//
// This file sets the process up and calls the entry package, app, which is its composition root.
// Nothing else in this component is package main.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ppat/mediated-mailbox-mcp/migrator/app"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
)

func main() {
	// Run sets the level once the configuration has loaded. The logger is the process default only
	// for code this project does not own, which logs through the default (ADR-0122).
	level := new(slog.LevelVar)
	logger := logging.New(os.Stdout, level)
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr, logger, level)
	stop()
	if err != nil {
		logger.Error("the migration failed", "error", err)
		os.Exit(1)
	}
}
