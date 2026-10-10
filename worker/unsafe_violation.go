//go:build banproof

package main

// This file imports unsafe on purpose, and uses reflect's unsafe pointers, which need no import of it.
// Every component list admits the standard library, unsafe and reflect included, so the go vet
// analyser for unsafe is the one refusing both, and no code of this project uses either (ADR-0117).
import (
	"reflect"
	_ "unsafe" // want vetcheck "imports unsafe"
)

func readsAnIntAsAFloat(v *int) float64 {
	p := reflect.ValueOf(v).UnsafePointer()                            // want vetcheck "uses one of reflect's unsafe pointers"
	_ = reflect.SliceAt(reflect.TypeFor[float64](), p, 1)              // want vetcheck "uses one of reflect's unsafe pointers"
	return reflect.NewAt(reflect.TypeFor[float64](), p).Elem().Float() // want vetcheck "uses one of reflect's unsafe pointers"
}
