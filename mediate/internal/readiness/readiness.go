package readiness

import "sync/atomic"

// State is whether the mediator is ready for traffic. The zero value is not ready.
type State struct {
	ready atomic.Bool
}

// MarkReady records that the policy has loaded and the listeners are up.
func (s *State) MarkReady() { s.ready.Store(true) }

// MarkNotReady records that the mediator has begun shutting down.
func (s *State) MarkNotReady() { s.ready.Store(false) }

// Ready reports whether the mediator is ready for traffic.
func (s *State) Ready() bool { return s.ready.Load() }
