package unsafeuser

import "reflect"

// punned reads an int's memory as a float through reflect's unsafe pointers, with no import of unsafe.
func punned(v *int) float64 {
	p := reflect.ValueOf(v).UnsafePointer()           // want "uses one of reflect's unsafe pointers"
	f := reflect.NewAt(reflect.TypeFor[float64](), p) // want "uses one of reflect's unsafe pointers"
	reflect.ValueOf(&p).Elem().SetPointer(p)          // want "uses one of reflect's unsafe pointers"
	get := reflect.ValueOf(v).UnsafePointer           // want "uses one of reflect's unsafe pointers"
	_ = reflect.Value.UnsafePointer                   // want "uses one of reflect's unsafe pointers"
	_ = get
	_ = reflect.SliceAt(reflect.TypeFor[float64](), p, 1) // want "uses one of reflect's unsafe pointers"
	return f.Elem().Float()
}

// safe uses reflect without its unsafe pointers, which the rule leaves alone.
func safe(v *int) uintptr {
	return reflect.ValueOf(v).Pointer()
}
