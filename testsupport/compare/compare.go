// Package compare holds the one comparison options value every Go unit test passes to cmp.Diff.
// The golden-file helper belongs here too. It compares and writes files under a Go package's own
// testdata/golden directory only. It does not yet reach the UI's recorded browser fixtures, which
// live beside the browser tests instead. Whether and how it serves them is for the ticket that
// first records one.
//
// Permission to read a type's unexported fields is granted here and nowhere else, one entry per
// type in readableTypes. The entries are fully qualified type names rather than type values, so
// this package imports none of the packages whose types it lists. That keeps two things possible
// that listing type values would forbid. An in-package test of any listed package can import this
// package without an import cycle, and a type under a deployable's internal directory can be
// listed at all. The cost is that the compiler does not check the names, so the resolving test in
// this package fails when an entry names no type.
//
// A type missing from the list makes cmp.Diff panic with a message naming the type. That is the
// intended signal, and the remedy is an entry here, never cmpopts.IgnoreUnexported.
package compare

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
)

// Options is the shared comparison options value. Pass it to every cmp.Diff call.
var Options = options(readableTypes)

// readableTypes lists, by import path and type name, every type whose unexported fields a test may
// read. It is empty until the first such type exists.
var readableTypes = map[string]struct{}{}

// options builds a comparison options value granting access to the unexported fields of the listed
// types.
func options(types map[string]struct{}) cmp.Options {
	return cmp.Options{cmp.Exporter(func(t reflect.Type) bool {
		_, ok := types[qualifiedName(t)]
		return ok
	})}
}

func qualifiedName(t reflect.Type) string {
	return t.PkgPath() + "." + t.Name()
}
