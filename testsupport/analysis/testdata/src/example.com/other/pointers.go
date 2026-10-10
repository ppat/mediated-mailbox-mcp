package other

import (
	"reflect"
	"unsafe"
)

// Pointer imports unsafe and uses reflect's unsafe pointers outside this module, where the rule on
// unsafe does not reach.
func Pointer(v *int) unsafe.Pointer { return reflect.ValueOf(v).UnsafePointer() }
