// Package schedule stands for the process family's scheduler, which starts every goroutine of the
// worker's jobs from outside the worker's code, so the rule over the worker does not read it.
package schedule

import "time"

func Go(f func()) {
	go f()
}

func After(f func()) {
	time.AfterFunc(time.Second, f)
}
