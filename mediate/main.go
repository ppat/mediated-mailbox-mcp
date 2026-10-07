// Command mediate is the mediator, published as mediated-mailbox-mediate.
//
// This file sets the process up and calls the deployable's entry package, app, which is its
// composition root and constructs the object graph by hand in ordinary code. Nothing else in this
// component is package main. Every other package of this deployable apart from app, and importtarget
// under the banproof tag, sits under internal, so the compiler refuses an import of it from any other
// component.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ppat/mediated-mailbox-mcp/mediate/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], os.Environ(), slog.Default())
	stop()
	if err != nil {
		slog.Error("the mediator stopped", "error", err)
		os.Exit(1)
	}
}
