package property

import (
	"runtime"

	"pgregory.net/rapid"
)

// fails reports whether prop fails on a, run outside rapid against a probe that records a failure.
// A panic counts as a failure.
func fails[A any](name string, prop func(rapid.TB, A), a A) bool {
	p := &probe{name: name}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recover() != nil {
				p.failed = true
			}
		}()
		prop(p, a)
	}()
	<-done
	return p.failed
}

// probe is a rapid.TB that records whether the property failed. A fatal call ends the goroutine
// the property runs on, as it does under testing and rapid.
type probe struct {
	name   string
	failed bool
}

func (p *probe) Helper()               {}
func (p *probe) Name() string          { return p.name }
func (p *probe) Logf(string, ...any)   {}
func (p *probe) Log(...any)            {}
func (p *probe) Skipf(string, ...any)  { runtime.Goexit() }
func (p *probe) Skip(...any)           { runtime.Goexit() }
func (p *probe) SkipNow()              { runtime.Goexit() }
func (p *probe) Errorf(string, ...any) { p.failed = true }
func (p *probe) Error(...any)          { p.failed = true }
func (p *probe) Fatalf(string, ...any) { p.failed = true; runtime.Goexit() }
func (p *probe) Fatal(...any)          { p.failed = true; runtime.Goexit() }
func (p *probe) FailNow()              { p.failed = true; runtime.Goexit() }
func (p *probe) Fail()                 { p.failed = true }
func (p *probe) Failed() bool          { return p.failed }
