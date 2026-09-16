//go:build banproof

// Package importtarget exists only when the banproof tag is set, as the mediator's list's target.
package importtarget

// A violation file must state a want, so this one also imports rapid into code that ships.
import (
	_ "pgregory.net/rapid" // want depguard "list 'non-test-code'"
)
