package heldcalls

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

// A test file may call either held method, since a call a test makes is never served.
func inATest(ctx context.Context, s *sessions.Session, c *connections.Connections) error {
	_ = c.Reseal(ctx)
	return s.Attach(c)
}
