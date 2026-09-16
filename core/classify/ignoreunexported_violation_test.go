//go:build banproof

package classify_test

import (
	"testing"

	co "github.com/google/go-cmp/cmp/cmpopts"
)

// This file uses the banned comparison option on purpose, through an aliased import.
func TestIgnoreUnexported(t *testing.T) {
	_ = co.IgnoreUnexported(struct{}{}) // want forbidigo `use of .co\.IgnoreUnexported. forbidden because "it reports no difference`
}
