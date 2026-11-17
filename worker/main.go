// Command worker is the worker, published as mediated-mailbox-worker. It runs every background job
// kind, backfill and delta sync among them (ADR-0117).
//
// This file hands the process the program's arguments, its environment and its spec, since only a
// main.go may read the environment (ADR-0078, ADR-0142). Nothing else in this component is package
// main.
package main

import (
	"os"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/worker/app"
)

func main() {
	process.Run(os.Args[1:], os.Environ(), app.Spec())
}
