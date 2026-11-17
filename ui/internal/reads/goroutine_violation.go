//go:build banproof

package reads

import (
	"net/http"
	"runtime"
)

// This file starts goroutines in the UI's code on purpose, by a go statement and through
// standard-library functions that run a function they are given on a goroutine of their own. The
// process would neither recover a panic on them nor wait for them before it closes its slots, so the
// UI's code starts one only through the process's Go (ADR-0142).
func startsAGoroutine(done chan struct{}, f func()) {
	go close(done) // want vetcheck "starts a goroutine outside the host's starter"
	p := new(int)
	runtime.SetFinalizer(p, func(*int) { f() }) // want vetcheck "starts a goroutine outside the host's starter"
	var server http.Server
	server.RegisterOnShutdown(f) // want vetcheck "starts a goroutine outside the host's starter"
}
