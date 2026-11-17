// Package connections stands for the connections, the one package that may attach them to the
// Session, and whose Reseal is held to delta sync's package.
package connections

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
)

type Scan struct{ Accounts map[string]bool }

type Connections struct{}

func (*Connections) Connected(string) bool { return false }

func (*Connections) Reseal(context.Context) Scan { return Scan{} }

func Of(s *sessions.Session) (*Connections, error) {
	c := &Connections{}
	return c, s.Attach(c)
}
