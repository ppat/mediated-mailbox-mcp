//go:build banproof

package main // want vetcheck "links to github.com/ppat/mediated-mailbox-mcp/core/sensitivity.ErrBodyWithheld in a pure core"

import (
	_ "unsafe"

	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
)

// This file reaches a pure core's error value from the composition root on purpose, and banproof
// requires the wants below from go vet. With the value nil, NewBody would refuse a withheld body
// with no error, and a caller would take the empty body as released.
func init() {
	sensitivity.ErrBodyWithheld = nil // want vetcheck "writes ErrBodyWithheld, a pure core's package-level variable"
}

//go:linkname bodyWithheld github.com/ppat/mediated-mailbox-mcp/core/sensitivity.ErrBodyWithheld
var bodyWithheld error
