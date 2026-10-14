//go:build banproof

package api

import (
	"net/http"
	"runtime"
)

// This file starts goroutines in the UI's code on purpose, by a go statement and through
// standard-library functions that run a function they are given on a goroutine of their own. The host
// would neither recover a panic on them nor wait for them before the pools close, so the UI starts one
// only through the starter the host hands its assembly (ADR-0138).
func startsAGoroutine(done chan struct{}, f func()) {
	go close(done) // want vetcheck "starts a goroutine outside the host's starter"
	p := new(int)
	runtime.SetFinalizer(p, func(*int) { f() }) // want vetcheck "starts a goroutine outside the host's starter"
	var server http.Server
	server.RegisterOnShutdown(f) // want vetcheck "starts a goroutine outside the host's starter"
}
