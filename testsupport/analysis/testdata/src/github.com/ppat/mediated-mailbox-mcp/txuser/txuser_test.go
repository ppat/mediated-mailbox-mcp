package txuser

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
)

// A test file is held to the rule too.
func sealedInATest(p pool) error {
	return credential.New(p).Sealed(context.Background()) // want `calls credential.New outside a function literal passed to tx.Run`
}
