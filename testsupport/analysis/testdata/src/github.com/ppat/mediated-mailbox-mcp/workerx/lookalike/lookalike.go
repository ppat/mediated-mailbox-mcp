// Package lookalike sits in a directory whose name only starts with worker, so it is not the worker's.
package lookalike

import "sync"

func start(f func()) {
	go f()
	var wg sync.WaitGroup
	wg.Go(f)
}
