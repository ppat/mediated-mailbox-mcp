//go:build banproof

package reload

import (
	"context"
	"sync"
	"time"
)

// This file starts goroutines in the API server's code on purpose, by a go statement and through
// standard-library functions that run a function they are given on a goroutine of their own. The host
// would neither recover a panic on them nor wait for them before the pools close, so the API server
// starts one only through the starter the host hands its assembly (ADR-0138).
func startsAGoroutine(ctx context.Context, done chan struct{}, f func()) {
	go close(done) // want vetcheck "starts a goroutine outside the host's starter"
	var wg sync.WaitGroup
	wg.Go(f)                       // want vetcheck "starts a goroutine outside the host's starter"
	time.AfterFunc(time.Second, f) // want vetcheck "starts a goroutine outside the host's starter"
	context.AfterFunc(ctx, f)      // want vetcheck "starts a goroutine outside the host's starter"
}
