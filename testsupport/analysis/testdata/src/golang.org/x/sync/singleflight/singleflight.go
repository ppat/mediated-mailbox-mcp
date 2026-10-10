// Package singleflight stands for golang.org/x/sync/singleflight, whose DoChan runs the function it is
// given on a goroutine of its own and whose Do runs it on the caller's.
package singleflight

type Result struct{}

type Group struct{}

func (g *Group) DoChan(key string, fn func() (any, error)) <-chan Result {
	go fn()
	return nil
}

func (g *Group) Do(key string, fn func() (any, error)) (any, error, bool) {
	v, err := fn()
	return v, err, false
}
