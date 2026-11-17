// Package offromui asks for the connections from the UI's configuration, which must not compile, since
// the UI's configuration type has the credential part and no accounts part.
package offromui

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

func Build(ctx context.Context, p process.Capabilities, c config.UI, s *sessions.Session) (*connections.Connections, error) {
	return connections.Of(ctx, p, c, s)
}
