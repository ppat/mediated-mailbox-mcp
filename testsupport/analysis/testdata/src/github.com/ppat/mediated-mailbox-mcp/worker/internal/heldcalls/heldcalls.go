// Package heldcalls stands for code outside both held packages, which reaches each held method as a
// call, a method value, a method expression and through an interface or a type parameter of the
// same signature, beside lookalikes that are not held.
package heldcalls

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

type resealer interface {
	Reseal(context.Context) connections.Scan
}

type attacher interface {
	Attach(sessions.Connections) error
}

// otherReseal has the held method's name and another signature, so it is not held.
type otherReseal interface {
	Reseal(context.Context) error
}

// Session is a lookalike of the Session in another package, whose Attach is not held.
type Session struct{}

func (Session) Attach(sessions.Connections) error { return nil }

func Calls(ctx context.Context, s *sessions.Session, c *connections.Connections, r resealer, a attacher, o otherReseal) {
	_ = c.Reseal(ctx)                     // want `uses the connections' Reseal outside delta sync's package`
	value := c.Reseal                     // want `uses the connections' Reseal outside delta sync's package`
	_ = (*connections.Connections).Reseal // want `uses the connections' Reseal outside delta sync's package`
	_ = s.Attach(c)                       // want `uses the Session's Attach outside process/connections`
	attach := (*sessions.Session).Attach  // want `uses the Session's Attach outside process/connections`
	_ = r.Reseal(ctx)                     // want `uses the connections' Reseal outside delta sync's package`
	_ = a.Attach(c)                       // want `uses the Session's Attach outside process/connections`
	_ = o.Reseal(ctx)
	_ = Session{}.Attach(c)
	_, _ = value, attach
}

func Generic[T resealer](ctx context.Context, t T) connections.Scan {
	return t.Reseal(ctx) // want `uses the connections' Reseal outside delta sync's package`
}
