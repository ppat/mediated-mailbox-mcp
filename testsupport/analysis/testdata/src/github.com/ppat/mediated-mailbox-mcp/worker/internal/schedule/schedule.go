// Package schedule stands for the worker's scheduler, which starts every goroutine of its jobs.
package schedule

import "time"

func Go(f func()) {
	go f()
}

func After(f func()) {
	time.AfterFunc(time.Second, f)
}
