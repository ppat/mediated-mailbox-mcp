// Package job stands for a job kind's code in the worker.
package job

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

func run(done chan struct{}) {
	go func() { close(done) }() // want "starts a goroutine outside schedule.Go"
	go close(done)              // want "starts a goroutine outside schedule.Go"
}

// embedded promotes WaitGroup's Go, which is refused as WaitGroup's own.
type embedded struct {
	sync.WaitGroup
}

func starters(ctx context.Context, f func()) {
	var wg sync.WaitGroup
	wg.Go(f)       // want "starts a goroutine outside schedule.Go"
	start := wg.Go // want "starts a goroutine outside schedule.Go"
	start(f)
	(*sync.WaitGroup).Go(&wg, f) // want "starts a goroutine outside schedule.Go"
	var e embedded
	e.Go(f)                        // want "starts a goroutine outside schedule.Go"
	time.AfterFunc(time.Second, f) // want "starts a goroutine outside schedule.Go"
	context.AfterFunc(ctx, f)      // want "starts a goroutine outside schedule.Go"
	p := new(int)
	runtime.SetFinalizer(p, func(*int) { f() }) // want "starts a goroutine outside schedule.Go"
	runtime.AddCleanup(p, func(int) { f() }, 0) // want "starts a goroutine outside schedule.Go"
	var server http.Server
	server.RegisterOnShutdown(f) // want "starts a goroutine outside schedule.Go"
	var group errgroup.Group
	group.Go(func() error { f(); return nil })    // want "starts a goroutine outside schedule.Go"
	group.TryGo(func() error { f(); return nil }) // want "starts a goroutine outside schedule.Go"
	var flight singleflight.Group
	flight.DoChan("k", func() (any, error) { f(); return nil, nil }) // want "starts a goroutine outside schedule.Go"
}

// callers runs functions on its own goroutine through APIs that start none, which the rule leaves
// alone.
func callers(f func()) {
	var once sync.Once
	once.Do(f)
	var flight singleflight.Group
	_, _, _ = flight.Do("k", func() (any, error) { f(); return nil, nil })
	var group errgroup.Group
	_ = group.Wait()
	timer := time.NewTimer(time.Second)
	timer.Stop()
}
