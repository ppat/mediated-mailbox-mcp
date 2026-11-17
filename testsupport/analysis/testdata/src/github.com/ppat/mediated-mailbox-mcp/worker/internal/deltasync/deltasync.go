// Package deltasync stands for delta sync's package, the one package that may re-seal, which may not
// attach connections.
package deltasync

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

func Tick(ctx context.Context, s *sessions.Session, c *connections.Connections) error {
	_ = c.Reseal(ctx)
	return s.Attach(c) // want `uses the Session's Attach outside process/connections`
}
