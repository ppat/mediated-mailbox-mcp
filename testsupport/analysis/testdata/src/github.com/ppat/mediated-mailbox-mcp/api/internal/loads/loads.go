// Package loads stands for the API server's code, which starts a goroutine only through the host's
// starter.
package loads

import (
	"context"
	"sync"
	"time"
)

func start(f func()) {
	go f() // want "starts a goroutine outside the host's starter"
	var wg sync.WaitGroup
	wg.Go(f)                                   // want "starts a goroutine outside the host's starter"
	time.AfterFunc(time.Second, f)             // want "starts a goroutine outside the host's starter"
	context.AfterFunc(context.Background(), f) // want "starts a goroutine outside the host's starter"
}
