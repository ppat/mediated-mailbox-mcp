// Package wiring stands for a composition root, which is not a pure core.
package wiring // want `links to github.com/ppat/mediated-mailbox-mcp/core/state.ErrRefused in a pure core`

import (
	"errors"
	_ "unsafe"

	"github.com/ppat/mediated-mailbox-mcp/core/state"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/core/plan"
)

// A composition root may hold its own variables.
var client = "c"

//go:linkname refused github.com/ppat/mediated-mailbox-mcp/core/state.ErrRefused
var refused error

// A link to a symbol outside every pure core is not this rule's.
//
//go:linkname ticks runtime.ticks
var ticks int

// Wire writes pure-core variables from outside, and its own variable, which it may.
func Wire() {
	state.ErrRefused = nil                                        // want `writes ErrRefused, a pure core's package-level variable`
	state.Fetch = func(string) (string, error) { return "", nil } // want `writes Fetch, a pure core's package-level variable`
	(state.ErrTyped) = errors.New("typed")                        // want `writes ErrTyped, a pure core's package-level variable`
	state.ErrSpoofed = nil                                        // want `writes ErrSpoofed, a pure core's package-level variable`
	state.Pair.A = 2                                              // want `writes Pair, a pure core's package-level variable`
	plan.Steps = append(plan.Steps, "s")                          // want `writes Steps, a pure core's package-level variable`
	_ = &state.ErrRefused                                         // want `writes ErrRefused, a pure core's package-level variable`
	client = "d"
	_ = refused
	_ = ticks
}
