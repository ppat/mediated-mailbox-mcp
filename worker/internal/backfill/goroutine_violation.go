//go:build banproof

package backfill

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// This file starts goroutines in a job kind's code on purpose, by a go statement and through each
// standard-library function that runs a function it is given on a goroutine of its own. A panic on any
// of them would stop the worker, since only the goroutine that panics can recover, so job code starts
// one only through schedule.Go (ADR-0119).
func startsAGoroutine(ctx context.Context, done chan struct{}, f func()) {
	go close(done) // want vetcheck "starts a goroutine outside schedule.Go"
	var wg sync.WaitGroup
	wg.Go(f)                       // want vetcheck "starts a goroutine outside schedule.Go"
	time.AfterFunc(time.Second, f) // want vetcheck "starts a goroutine outside schedule.Go"
	context.AfterFunc(ctx, f)      // want vetcheck "starts a goroutine outside schedule.Go"
	p := new(int)
	runtime.SetFinalizer(p, func(*int) { f() }) // want vetcheck "starts a goroutine outside schedule.Go"
	runtime.AddCleanup(p, func(int) { f() }, 0) // want vetcheck "starts a goroutine outside schedule.Go"
	var server http.Server
	server.RegisterOnShutdown(f) // want vetcheck "starts a goroutine outside schedule.Go"
}
