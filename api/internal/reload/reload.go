// Package reload runs a load each caller needs started after it asked, sharing one load among the
// callers waiting when it starts (ADR-0099). The API server loads the policy this way before each body
// request decides, so a policy edit written before the request arrived is in the snapshot the request
// takes, and a burst of requests shares one load rather than queueing one each.
//
// A load running when a caller asks may have read the source before the change the caller must see,
// so it never counts for that caller. The caller waits for the next load, which starts as soon as the
// running one ends and answers every caller that asked meanwhile.
//
// The loads run on a goroutine the starter Coalesced is built with starts, which in a process is the
// host's, so a panic in a load is the API server's failure and the goroutine never outlives the pools
// (ADR-0138).
package reload

import (
	"context"
	"errors"
	"sync"
)

// Starter starts run on a goroutine its owner recovers and joins, under a context that outlives every
// caller, and reports whether it did. The host's starter is one.
type Starter func(name string, run func(context.Context)) bool

// ErrNotStarted is a load's error when the starter started nothing, because the process is stopping.
var ErrNotStarted = errors.New("the load was not started, since the process is stopping")

// ErrPanicked is a load's error when the load panicked. The panic goes on to the starter's owner.
var ErrPanicked = errors.New("the load panicked")

// Coalesced runs load for its callers. Loads run one at a time, each under the context the starter
// gives, so a caller that stops waiting never cancels a load other callers wait on.
type Coalesced struct {
	start Starter
	load  func(context.Context) error

	mu      sync.Mutex
	next    *round
	running bool
}

// round is one load and the callers it answers.
type round struct {
	done chan struct{}
	err  error
}

// New returns a Coalesced running load on the goroutines start starts.
func New(start Starter, load func(context.Context) error) *Coalesced {
	return &Coalesced{start: start, load: load}
}

// loadName names the loads' goroutine to the starter.
const loadName = "coalesced load"

// Load waits for a load that starts after it was called and returns that load's error, or ctx's error
// when ctx ends first. The load goes on for the other callers waiting on it. A load the starter does
// not start returns ErrNotStarted.
func (c *Coalesced) Load(ctx context.Context) error {
	c.mu.Lock()
	r := c.next
	if r == nil {
		r = &round{done: make(chan struct{})}
		c.next = r
		if !c.running {
			if c.start(loadName, c.run) {
				c.running = true
			} else {
				c.next = nil
				r.err = ErrNotStarted
				close(r.done)
			}
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

// run starts each waiting round in turn, after the one before it ended, until none waits. A load that
// panics answers its round and every round waiting with ErrPanicked, and the panic goes on to the
// starter's owner, so no caller waits on a load that will never end.
func (c *Coalesced) run(ctx context.Context) {
	var r *round
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		c.mu.Lock()
		waiting := c.next
		c.next = nil
		c.running = false
		c.mu.Unlock()
		for _, w := range []*round{r, waiting} {
			if w != nil {
				w.err = ErrPanicked
				close(w.done)
			}
		}
		panic(p)
	}()
	for {
		c.mu.Lock()
		r = c.next
		if r == nil {
			c.running = false
			c.mu.Unlock()
			return
		}
		c.next = nil
		c.mu.Unlock()
		r.err = c.load(ctx)
		close(r.done)
		r = nil
	}
}
