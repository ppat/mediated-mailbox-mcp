//go:build banproof

package classify_test

import (
	"testing"

	r "pgregory.net/rapid"
)

// This file calls the reflection-based generators on purpose, through an aliased import, through a
// function value, and inside an Example function.
func TestReflectionGenerators(t *testing.T) {
	_ = r.Make[int]()                     // want forbidigo `use of .r\.Make. forbidden because "generators build values through exported constructors`
	_ = r.MakeCustom[int](r.MakeConfig{}) // want forbidigo `use of .r\.MakeCustom. forbidden because "generators build values through exported constructors`
	makeString := r.Make[string]          // want forbidigo `use of .r\.Make. forbidden because "generators build values through exported constructors`
	_ = makeString
}

func ExampleMake() {
	_ = r.Make[bool]() // want forbidigo `use of .r\.Make. forbidden because "generators build values through exported constructors`
}
