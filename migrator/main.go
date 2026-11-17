// Command migrator is the migrator, published as mediated-mailbox-migrator, which the migration step
// runs (ADR-0048, ADR-0139). It applies the migration chain its binary carries and exits.
//
// This file hands the process the program's arguments, its environment and its spec, since only a
// main.go may read the environment (ADR-0078, ADR-0142). Nothing else in this component is package
// main.
package main

import (
	"os"

	"github.com/ppat/mediated-mailbox-mcp/migrator/app"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

func main() {
	process.Run(os.Args[1:], os.Environ(), app.Spec())
}
