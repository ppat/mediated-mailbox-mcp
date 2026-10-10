package unsafeuser

import "reflect"

var testPointer = reflect.ValueOf(new(int)).UnsafePointer()
