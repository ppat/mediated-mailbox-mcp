// Command vetcheck runs the placement analyser under go vet.
//
//	go vet -vettool="$(go tool -n vetcheck)" ./...
package main

import (
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/analysis"
)

func main() { unitchecker.Main(analysis.Analyzer) }
