// Package streams stands for the UI's code, which starts a goroutine only through the host's starter.
package streams

import "sync"

func start(f func()) {
	go f() // want "starts a goroutine outside the host's starter"
	var wg sync.WaitGroup
	begin := wg.Go // want "starts a goroutine outside the host's starter"
	begin(f)
}
