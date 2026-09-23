// Package rapid stubs the parts of pgregory.net/rapid the analyser recognises.
package rapid

import "testing"

type T struct{}

type TB interface{ Fatalf(string, ...any) }

type Generator[V any] struct{}

func Check(t *testing.T, prop func(*T))                                  {}
func MakeCheck(prop func(*T)) func(*testing.T)                            { return nil }
func Custom[V any](fn func(*T) V) *Generator[V]                           { return nil }
func Map[U, V any](g *Generator[U], fn func(U) V) *Generator[V]           { return nil }
func Deferred[V any](fn func() *Generator[V]) *Generator[V]               { return nil }
func (g *Generator[V]) Filter(fn func(V) bool) *Generator[V]              { return g }
func (t *T) Repeat(actions map[string]func(*T))                           {}
