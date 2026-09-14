// Package newdir sits in a directory no component's import list names, and imports a module the
// residual list refuses.
package newdir

import (
	// Imported only so the residual list refuses it.
	_ "pgregory.net/rapid"
)
