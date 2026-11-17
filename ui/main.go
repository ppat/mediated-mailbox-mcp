// Command ui is the UI, published as mediated-mailbox-ui.
//
// This file hands the process the program's arguments, its environment and its spec, since only a
// main.go may read the environment (ADR-0078, ADR-0142). Nothing else in this component is package
// main. Every other package of this deployable apart from app and the browser bundle's package,
// browser, sits under internal, so the compiler refuses an import of it from any other component.
package main

import (
	"os"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/ui/app"
)

func main() {
	process.Run(os.Args[1:], os.Environ(), app.Spec())
}
