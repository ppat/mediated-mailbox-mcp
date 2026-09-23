// Command vetcheck runs the project's go vet analysers, placement and globals.
//
//	go vet -vettool="$(go tool -n vetcheck)" ./...
package main

import (
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/analysis"
)

func main() { unitchecker.Main(analysis.Placement, analysis.Globals) }
