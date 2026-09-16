// Package core is the shared pure library, published as mediated-mailbox-core.
//
// Everything under this directory is pure. Nothing here performs input or output, reads the
// environment or the clock, or holds a connection. Every dependency arrives as a parameter, and every
// decision is returned as a value that a shell enacts.
//
// Each shared pure concern is one subpackage directly below this directory. A subpackage imports only
// the standard-library packages the pure-core import list names and other pure-core packages. Nothing
// belongs in this package itself.
package core
