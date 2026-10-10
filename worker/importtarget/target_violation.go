//go:build banproof

// Package importtarget exists only when the banproof tag is set, as the mediator's list's target.
package importtarget

// A violation file must state a want, so this one also imports the comparison library tests use into
// code that ships.
import (
	_ "github.com/google/go-cmp/cmp" // want depguard "list 'non-test-code'"
)
