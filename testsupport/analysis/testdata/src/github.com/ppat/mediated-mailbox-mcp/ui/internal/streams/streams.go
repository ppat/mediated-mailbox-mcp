// Package streams stands for the UI's code, which starts a goroutine only through the process's Go.
package streams

import "sync"

func start(f func()) {
	go f() // want "starts a goroutine outside the process's Go"
	var wg sync.WaitGroup
	begin := wg.Go // want "starts a goroutine outside the process's Go"
	begin(f)
}
