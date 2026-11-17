//go:build banproof

// Package importtarget exists only when the banproof tag is set, as the API server's list's target.
package importtarget

// A violation file must state a want, so this one also imports the comparison library tests use into
// code that ships, which the worker's list over its non-test code and the list over all non-test code
// both refuse.
import (
	_ "github.com/google/go-cmp/cmp" // want depguard "list 'worker'" depguard "list 'non-test-code'"
)
