package reload_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/reload"
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

// starter returns a starter that runs each goroutine under the test's context and waits for every one
// when the test ends, as the host's does under the process's.
func starter(t *testing.T) reload.Starter {
	t.Helper()
	var running sync.WaitGroup
	t.Cleanup(running.Wait)
	return func(_ string, run func(context.Context)) bool {
		running.Go(func() { run(t.Context()) })
		return true
	}
}

// A load already running when a caller asks never answers it, since it may have read the source
// before the change the caller must see. The caller is answered by the next load, which starts once
// the running one ends (ADR-0099).
func TestALoadRunningWhenACallerAsksDoesNotCount(t *testing.T) {
	g := newGated()
	c := reload.New(starter(t), g.load)
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
	c := reload.New(starter(t), g.load)
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
// others, under the context the starter gives it, which the load watches.
func TestACallerThatStopsWaitingLeavesTheLoadRunning(t *testing.T) {
	g := newGated()
	c := reload.New(starter(t), g.load)
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
	c := reload.New(starter(t), func(context.Context) error {
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

// Every load runs on a goroutine the starter started, so in a process it is the host's to recover and
// to wait for before the pools close (ADR-0138).
func TestEveryLoadRunsThroughTheStarter(t *testing.T) {
	var inside, outside atomic.Int64
	var running sync.WaitGroup
	t.Cleanup(running.Wait)
	start := func(_ string, run func(context.Context)) bool {
		running.Go(func() {
			inside.Add(1)
			defer inside.Add(-1)
			run(t.Context())
		})
		return true
	}
	c := reload.New(start, func(context.Context) error {
		if inside.Load() == 0 {
			outside.Add(1)
		}
		return nil
	})
	for range 5 {
		if err := c.Load(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if n := outside.Load(); n != 0 {
		t.Errorf("%d loads ran on a goroutine the starter did not start", n)
	}
}

// A starter that starts nothing, as the host's does once the process stops, fails the caller at once
// rather than leaving it waiting for a load that will never run.
func TestARefusedStartFailsItsCallers(t *testing.T) {
	var loads atomic.Int64
	c := reload.New(func(string, func(context.Context)) bool { return false }, func(context.Context) error {
		loads.Add(1)
		return nil
	})
	for range 2 {
		if err := within(t, waiting(t, c)); !errors.Is(err, reload.ErrNotStarted) {
			t.Errorf("a caller got %v, want ErrNotStarted", err)
		}
	}
	if n := loads.Load(); n != 0 {
		t.Errorf("%d loads ran, though the starter started none", n)
	}
}

// A load that panics answers its callers and every caller waiting for the next load with ErrPanicked,
// and the panic reaches the starter's owner, which recovers it as the component's failure.
func TestAPanickedLoadFailsItsCallers(t *testing.T) {
	g := newGated()
	panicked := make(chan any, 1)
	var running sync.WaitGroup
	t.Cleanup(running.Wait)
	start := func(_ string, run func(context.Context)) bool {
		running.Go(func() {
			defer func() { panicked <- recover() }()
			run(t.Context())
		})
		return true
	}
	c := reload.New(start, func(ctx context.Context) error {
		if err := g.load(ctx); err != nil {
			panic(err)
		}
		return nil
	})
	first := waiting(t, c)
	within(t, g.started)
	second := waiting(t, c)
	settle()
	g.let(t, errors.New("the load broke"))
	for name, answer := range map[string]<-chan error{"the load's caller": first, "the waiting caller": second} {
		if err := within(t, answer); !errors.Is(err, reload.ErrPanicked) {
			t.Errorf("%s got %v, want ErrPanicked", name, err)
		}
	}
	if p := within(t, panicked); p == nil {
		t.Error("the starter's owner never saw the panic")
	}
}
