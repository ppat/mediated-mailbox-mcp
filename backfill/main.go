// Command backfill is backfill, published as mediated-mailbox-backfill.
//
// This file sets the process up and calls the deployable's entry package, app, which is its
// composition root and constructs the object graph by hand in ordinary code. Nothing else in this
// component is package main. Every other package of this deployable apart from app sits under
// internal, so the compiler refuses an import of it from any other component.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ppat/mediated-mailbox-mcp/backfill/app"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
)

func main() {
	// The entry sets the level once the configuration has loaded. The logger is the process default
	// only for code this project does not own, which logs through the default (ADR-0122).
	level := new(slog.LevelVar)
	logger := logging.New(os.Stdout, level)
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], os.Environ(), logger, level)
	stop()
	if err != nil {
		logger.Error("backfill stopped", "error", err)
		os.Exit(1)
	}
}
