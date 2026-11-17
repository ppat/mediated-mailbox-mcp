package process

import (
	"context"
	"errors"
	"sync"
)

// OnePerProcess is a slot: the place in each process where one owner package keeps the one thing of
// type T the process's endpoints and job kinds share, such as the Session or the scanner (ADR-0142).
// An owner declares one, once, with NewOnePerProcess, and its one function gets what the slot holds
// through Get. Nothing is looked up by type, name or reflection.
type OnePerProcess[T any] struct {
	name string
}

// NewOnePerProcess declares a slot whose thing reports its readiness, logs and registers under name.
func NewOnePerProcess[T any](name string) *OnePerProcess[T] {
	return &OnePerProcess[T]{name: name}
}

// slot is what one process holds in one slot.
type slot struct {
	mu    sync.Mutex
	built bool
	value any
}

// Get returns what the slot holds in the process p belongs to. The first call builds it under the
// slot's lock with build, and every later call returns the same one. build is handed the process's
// own capabilities, labelled with the program's name and no job kind and reporting under the slot's
// name, never those of whoever asked, so what the thing registers, logs and reports does not depend on
// which part asked first. It returns the thing and the function that closes it, nil for none, which
// the process calls, last built first, once its servers have drained and its jobs have ended. A build
// that fails keeps nothing, and the next call builds again. A build must not get its own slot.
func (s *OnePerProcess[T]) Get(ctx context.Context, p Capabilities, build func(ctx context.Context, shared Capabilities) (T, func(), error)) (T, error) {
	var zero T
	c, ok := p.(*capabilities)
	if !ok {
		return zero, errors.New("the capabilities were not handed out by a process")
	}
	pr := c.proc
	pr.slotsMu.Lock()
	held, ok := pr.slots[s]
	if !ok {
		held = &slot{}
		pr.slots[s] = held
	}
	pr.slotsMu.Unlock()

	held.mu.Lock()
	defer held.mu.Unlock()
	if v, ok := held.value.(T); ok && held.built {
		return v, nil
	}
	v, closeSlot, err := build(ctx, pr.part(s.name, ""))
	if err != nil {
		return zero, err
	}
	held.value, held.built = v, true
	if closeSlot != nil {
		pr.slotsMu.Lock()
		pr.closers = append(pr.closers, closeSlot)
		pr.slotsMu.Unlock()
	}
	return v, nil
}
