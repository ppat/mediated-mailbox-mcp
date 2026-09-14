//go:build banproof

package classify_test

import "testing"

func fails() error { return nil }

// This file discards an error and leaves a type assertion unchecked on purpose.
func TestUncheckedErrors(t *testing.T) {
	_ = fails() // want errcheck "Error return value is not checked"
	var v any = 1
	n := v.(int) // want errcheck "Error return value is not checked"
	_ = n
}
