// Package publicasprivate passes a public key where the opening subsection takes a private key,
// which must not compile.
package publicasprivate

import (
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
)

var public seal.PublicKey

var _, _ = open.NewKeyring(public, public)
