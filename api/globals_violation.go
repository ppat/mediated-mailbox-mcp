//go:build banproof

package main // want vetcheck "links to github.com/ppat/mediated-mailbox-mcp/core/sensitivity.ErrFlagsWithoutScan in a pure core"

import (
	_ "unsafe" // want vetcheck "imports unsafe"

	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// This file reaches a pure core's error value from the API server's package main on purpose, and
// banproof requires the wants below from go vet. With the value nil, New would refuse content flags
// on a message that was not scanned with no error, and a caller would take the zero Sensitivity it
// returns as one that was built.
func init() {
	sensitivity.ErrFlagsWithoutScan = nil // want vetcheck "writes ErrFlagsWithoutScan, a pure core's package-level variable"
}

//go:linkname flagsWithoutScan github.com/ppat/mediated-mailbox-mcp/core/sensitivity.ErrFlagsWithoutScan
var flagsWithoutScan error
