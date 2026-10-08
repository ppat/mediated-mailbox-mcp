// Package nested sits in a directory named core at depth three, which is a pure core.
package nested

var Handler func() // want `a pure core declares no package-level variable other than an error value`
