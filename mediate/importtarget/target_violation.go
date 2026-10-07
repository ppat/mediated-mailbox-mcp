//go:build banproof

// Package importtarget exists only when the banproof tag is set. It is a package outside internal
// in a deployable, so that the other deployables' import lists have something to refuse. The layout
// allows one other package there, the deployable's entry package app, which the import lists refuse
// to every other component in the same way. Every other package of a deployable sits under
// internal, so the compiler refuses an import of it instead.
package importtarget

// This import proves the mediator's own list refuses another deployable.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/propose/importtarget" // want depguard "list 'mediate'"
)
