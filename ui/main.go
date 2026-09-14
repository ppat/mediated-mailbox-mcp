// Command ui is the UI, published as mediated-mailbox-ui.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
//
// The browser bundle is embedded here and passed into the server, so no other package reaches for it.
// The pattern carries the all: prefix because a fresh clone holds only the checked-in placeholder in
// browser/dist, and without the prefix a directory holding only a dot file fails to compile. The
// image build refuses a bundle that is only the placeholder (ui/Dockerfile).
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

//go:embed all:browser/dist
var embedded embed.FS

func main() {
	bundle, err := fs.Sub(embedded, "browser/dist")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ui:", err)
		os.Exit(1)
	}
	_ = api.New(bundle)
}
