package reload_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/reload"
)

// gated is a load each run of which starts, reports its number, and waits to be let finish with the
// error it is given.
type gated struct {
	started chan int
	finish  chan error
	runs    atomic.Int64
}

func newGated() *gated { return &gated{started: make(chan int), finish: make(chan error)} }

func (g *gated) load(ctx context.Context) error {
	n := int(g.runs.Add(1))
	select {
	case g.started <- n:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-g.finish:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// within returns what ch delivers, failing the test when nothing arrives in five seconds, so a load
// that never starts or a caller that is never answered fails rather than hangs.
func within[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("nothing arrived in five seconds")
	}
	var zero T
	return zero
}

// let lets the running load finish with err, failing the test when no load takes it.
func (g *gated) let(t *testing.T, err error) {
	t.Helper()
	select {
	case g.finish <- err:
	case <-time.After(5 * time.Second):
		t.Fatal("no load was running to finish")
	}
}

// waiting starts a caller of Load and returns where its answer arrives.
func waiting(t *testing.T, c *reload.Coalesced) <-chan error {
	t.Helper()
	out := make(chan error, 1)
	go func() { out <- c.Load(t.Context()) }()
	return out
}

// settle gives the callers started just before time to reach Load.
func settle() { time.Sleep(50 * time.Millisecond) }

// A load already running when a caller asks never answers it, since it may have read the source
// before the change the caller must see. The caller is answered by the next load, which starts once
// the running one ends (ADR-0099).
func TestALoadRunningWhenACallerAsksDoesNotCount(t *testing.T) {
	g := newGated()
	c := reload.New(t.Context(), g.load)
	first := waiting(t, c)
	if n := within(t, g.started); n != 1 {
		t.Fatalf("the first load is number %d", n)
	}
	second := waiting(t, c)
	settle()
	g.let(t, errors.New("the first load failed"))
	if err := within(t, first); err == nil {
		t.Error("the first caller was not answered by the load it waited on")
	}
	select {
	case err := <-second:
		t.Fatalf("the caller that asked while the first load ran was answered by it, with %v", err)
	case n := <-g.started:
		if n != 2 {
			t.Fatalf("the next load is number %d", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no load started for the caller that asked while the first ran")
	}
	g.let(t, nil)
	if err := within(t, second); err != nil {
		t.Errorf("the second caller got %v from the load that started after it asked", err)
	}
}

// Every caller waiting when a load starts shares it, so a burst of callers runs one load, not one
// each.
func TestCallersWaitingTogetherShareOneLoad(t *testing.T) {
	g := newGated()
	c := reload.New(t.Context(), g.load)
	first := waiting(t, c)
	within(t, g.started)
	var answers []<-chan error
	for range 5 {
		answers = append(answers, waiting(t, c))
	}
	settle()
	g.let(t, nil)
	if err := within(t, first); err != nil {
		t.Errorf("the first caller got %v", err)
	}
	within(t, g.started)
	failed := errors.New("the shared load failed")
	g.let(t, failed)
	for _, a := range answers {
		if err := within(t, a); !errors.Is(err, failed) {
			t.Errorf("a waiting caller got %v, want the shared load's error", err)
		}
	}
	if n := g.runs.Load(); n != 2 {
		t.Errorf("%d loads ran, want 2", n)
	}
}

// A caller that stops waiting returns its own context's error and leaves the load running for the
// others, under the context Coalesced was built with, which the load watches.
func TestACallerThatStopsWaitingLeavesTheLoadRunning(t *testing.T) {
	g := newGated()
	c := reload.New(t.Context(), g.load)
	ctx, cancel := context.WithCancel(t.Context())
	gone := make(chan error, 1)
	go func() { gone <- c.Load(ctx) }()
	within(t, g.started)
	other := waiting(t, c)
	settle()
	cancel()
	if err := within(t, gone); !errors.Is(err, context.Canceled) {
		t.Errorf("the caller that stopped waiting got %v", err)
	}
	g.let(t, nil)
	within(t, g.started)
	g.let(t, nil)
	if err := within(t, other); err != nil {
		t.Errorf("the other caller got %v", err)
	}
}

// Loads never overlap, whatever the callers do.
func TestLoadsRunOneAtATime(t *testing.T) {
	var running, most atomic.Int64
	c := reload.New(t.Context(), func(context.Context) error {
		n := running.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
		return nil
	})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if err := c.Load(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if m := most.Load(); m != 1 {
		t.Errorf("%d loads ran at once", m)
	}
}
