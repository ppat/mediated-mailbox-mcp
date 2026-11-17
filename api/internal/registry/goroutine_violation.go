//go:build banproof

package registry

import (
	"context"
	"sync"
	"time"
)

// This file starts goroutines in the API server's code on purpose, by a go statement and through
// standard-library functions that run a function they are given on a goroutine of their own. The
// process would neither recover a panic on them nor wait for them before the slots close, so the API
// server starts one only through the process's Go (ADR-0142).
func startsAGoroutine(ctx context.Context, done chan struct{}, f func()) {
	go close(done) // want vetcheck "starts a goroutine outside the process's Go"
	var wg sync.WaitGroup
	wg.Go(f)                       // want vetcheck "starts a goroutine outside the process's Go"
	time.AfterFunc(time.Second, f) // want vetcheck "starts a goroutine outside the process's Go"
	context.AfterFunc(ctx, f)      // want vetcheck "starts a goroutine outside the process's Go"
}
