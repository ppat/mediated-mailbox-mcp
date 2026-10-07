// Command ui is the UI, published as mediated-mailbox-ui.
//
// This file sets the process up and calls the deployable's entry package, app, which is its
// composition root and constructs the object graph by hand in ordinary code. Nothing else in this
// component is package main. Every other package of this deployable apart from app sits under
// internal, so the compiler refuses an import of it from any other component.
//
// The browser bundle is embedded here, since an embed pattern cannot reach a parent directory, and
// passed into the entry, which hands it to the server, so no other package reaches for it. The
// pattern carries the all: prefix because a fresh clone holds only the checked-in placeholder in
// browser/dist, and without the prefix a directory holding only a dot file fails to compile. The image
// build refuses a bundle that is only the placeholder (ui/Dockerfile).
package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ppat/mediated-mailbox-mcp/ui/app"
)

//go:embed all:browser/dist
var embedded embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	err := app.Run(ctx, os.Args[1:], os.Environ(), logger, embedded)
	stop()
	if err != nil {
		logger.Error("ui stopped", "error", err)
		os.Exit(1)
	}
}
