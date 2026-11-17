// Package sessions stands for the Session, whose Attach is held to the connections' package.
package sessions

type Connections interface{ Connected(account string) bool }

type Session struct{ c Connections }

func (s *Session) Attach(c Connections) error { s.c = c; return nil }
