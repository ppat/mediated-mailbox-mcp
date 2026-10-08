// Package lookalike sits under a directory whose name ends with core, which is not a pure core.
package lookalike

var Handler func()

func Set() { Handler = func() {} }
