// Package unsafeuser stands for project code that imports unsafe.
package unsafeuser

import (
	"unsafe" // want "imports unsafe"
)

var size = unsafe.Sizeof(0)
