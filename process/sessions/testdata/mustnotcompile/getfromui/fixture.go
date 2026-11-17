// Package getfromui asks for the Session from the UI's configuration, which must not compile, since
// the UI's configuration type has no accounts part.
package getfromui

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

func Build(ctx context.Context, p process.Capabilities, c config.UI) (*sessions.Session, error) {
	return sessions.Get(ctx, p, c)
}
