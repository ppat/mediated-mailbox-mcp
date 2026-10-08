// Package opensacredential asks the client-secret package to open a value bound to an account
// credential's context, which must not compile, since its one operation takes a client's name and no
// context.
package opensacredential

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
)

var opener *clientsecret.Opener

var _, _ = opener.Secret(context.Background(), seal.AccountCredential("personal"))
