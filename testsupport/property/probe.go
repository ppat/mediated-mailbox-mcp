package property

import (
	"fmt"
	"runtime"
	"strings"

	"pgregory.net/rapid"
)

// fails reports whether prop fails on a, run outside rapid against a recorder that stands in for a
// test. A panic counts as a failure.
func fails[A any](name string, prop func(rapid.TB, A), a A) bool {
	failed, _ := Probe(name, func(tb rapid.TB) { prop(tb, a) })
	return failed
}

// Probe runs f against a recorder standing in for a test, and reports whether f failed, a panic
// included, and what it logged. A fatal call ends f's goroutine, as it does under testing and rapid.
func Probe(name string, f func(rapid.TB)) (failed bool, log string) {
	r := &recorder{name: name}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if v := recover(); v != nil {
				r.failed = true
				r.log = append(r.log, fmt.Sprint("panic: ", v))
			}
		}()
		f(r)
	}()
	<-done
	return r.failed, strings.Join(r.log, "\n")
}

// recorder is the rapid.TB Probe runs f against.
type recorder struct {
	name   string
	failed bool
	log    []string
}

func (r *recorder) Helper()                       {}
func (r *recorder) Name() string                  { return r.name }
func (r *recorder) Logf(format string, a ...any)  { r.log = append(r.log, fmt.Sprintf(format, a...)) }
func (r *recorder) Log(a ...any)                  { r.log = append(r.log, fmt.Sprint(a...)) }
func (r *recorder) Skipf(format string, a ...any) { r.Logf(format, a...); runtime.Goexit() }
func (r *recorder) Skip(a ...any)                 { r.Log(a...); runtime.Goexit() }
func (r *recorder) SkipNow()                      { runtime.Goexit() }
func (r *recorder) Errorf(format string, a ...any) {
	r.failed = true
	r.Logf(format, a...)
}
func (r *recorder) Error(a ...any) { r.failed = true; r.Log(a...) }
func (r *recorder) Fatalf(format string, a ...any) {
	r.failed = true
	r.Logf(format, a...)
	runtime.Goexit()
}
func (r *recorder) Fatal(a ...any) { r.failed = true; r.Log(a...); runtime.Goexit() }
func (r *recorder) FailNow()       { r.failed = true; runtime.Goexit() }
func (r *recorder) Fail()          { r.failed = true }
func (r *recorder) Failed() bool   { return r.failed }
