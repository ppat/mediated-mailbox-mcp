// Package loads stands for the API server's code, which starts a goroutine only through the
// process's Go.
package loads

import (
	"context"
	"sync"
	"time"
)

func start(f func()) {
	go f() // want "starts a goroutine outside the process's Go"
	var wg sync.WaitGroup
	wg.Go(f)                                   // want "starts a goroutine outside the process's Go"
	time.AfterFunc(time.Second, f)             // want "starts a goroutine outside the process's Go"
	context.AfterFunc(context.Background(), f) // want "starts a goroutine outside the process's Go"
}
