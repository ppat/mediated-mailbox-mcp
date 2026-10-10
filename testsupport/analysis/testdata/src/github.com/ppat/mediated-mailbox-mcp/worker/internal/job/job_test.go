package job

import "sync"

func helper(done chan struct{}) {
	go close(done)
	var wg sync.WaitGroup
	wg.Go(func() { close(done) })
}
