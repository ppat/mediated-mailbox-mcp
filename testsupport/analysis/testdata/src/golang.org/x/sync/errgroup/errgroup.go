// Package errgroup stands for golang.org/x/sync/errgroup, whose Group runs each function it is given
// on a goroutine of its own.
package errgroup

type Group struct{}

func (g *Group) Go(f func() error) { go f() }

func (g *Group) TryGo(f func() error) bool { go f(); return true }

func (g *Group) Wait() error { return nil }
