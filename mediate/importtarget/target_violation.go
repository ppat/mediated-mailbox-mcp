//go:build banproof

// Package importtarget exists only when the banproof tag is set. It is a package outside internal
// in a deployable, which the layout forbids, so that the other deployables' import lists have
// something to refuse. Any other package of a deployable is refused by the compiler instead.
package importtarget

// This import proves the mediator's own list refuses another deployable.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/propose/importtarget" // want depguard "list 'mediate'"
)
