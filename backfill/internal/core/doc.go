// Package core is the directory for pure-core packages private to backfill.
//
// Each concern is one subpackage directly below this directory. The pure-core import list governs
// every file under it, so a package here imports only the standard-library packages that list names
// and other pure-core packages. Nothing belongs in this package itself.
package core
