// Package reload runs a load each caller needs started after it asked, sharing one load among the
// callers waiting when it starts (ADR-0099). The mediator loads the policy this way before each body
// request decides, so a policy edit written before the request arrived is in the snapshot the request
// takes, and a burst of requests shares one load rather than queueing one each.
//
// A load running when a caller asks may have read the source before the change the caller must see,
// so it never counts for that caller. The caller waits for the next load, which starts as soon as the
// running one ends and answers every caller that asked meanwhile.
package reload

import (
	"context"
	"sync"
)

// Coalesced runs load for its callers. Loads run one at a time, each under the context Coalesced was
// built with, so a caller that stops waiting never cancels a load other callers wait on.
type Coalesced struct {
	// ctx is the loads' context, which outlives every caller.
	ctx  context.Context
	load func(context.Context) error

	mu      sync.Mutex
	next    *round
	running bool
}

// round is one load and the callers it answers.
type round struct {
	done chan struct{}
	err  error
}

// New returns a Coalesced running load under ctx, which ends every load when it ends.
func New(ctx context.Context, load func(context.Context) error) *Coalesced {
	return &Coalesced{ctx: ctx, load: load}
}

// Load waits for a load that starts after it was called and returns that load's error, or ctx's error
// when ctx ends first. The load goes on for the other callers waiting on it.
func (c *Coalesced) Load(ctx context.Context) error {
	c.mu.Lock()
	r := c.next
	if r == nil {
		r = &round{done: make(chan struct{})}
		c.next = r
		if !c.running {
			c.running = true
			go c.run()
		}
	}
	c.mu.Unlock()
	select {
	case <-r.done:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run starts each waiting round in turn, after the one before it ended, until none waits.
func (c *Coalesced) run() {
	for {
		c.mu.Lock()
		r := c.next
		if r == nil {
			c.running = false
			c.mu.Unlock()
			return
		}
		c.next = nil
		c.mu.Unlock()
		r.err = c.load(c.ctx)
		close(r.done)
	}
}
