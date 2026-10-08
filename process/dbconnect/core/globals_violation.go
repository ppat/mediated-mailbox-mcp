//go:build banproof

package core

// This file gives a pure core whose core path element sits at depth three, process/dbconnect/core,
// package-level state on purpose, and banproof requires the want below from go vet. It proves the
// globals analyser matches a pure core at any depth.

var Defaults = Config{} // want vetcheck "a pure core declares no package-level variable other than an error value"
