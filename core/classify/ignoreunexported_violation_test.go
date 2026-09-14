//go:build banproof

package classify_test

import (
	"testing"

	co "github.com/google/go-cmp/cmp/cmpopts"
)

// This file uses the banned comparison option on purpose, through an aliased import, and then
// silences the same call with a suppression directive, which the directive search must report.
func TestIgnoreUnexported(t *testing.T) {
	_ = co.IgnoreUnexported(struct{}{}) // want forbidigo `use of .co\.IgnoreUnexported. forbidden because "it reports no difference`
	_ = co.IgnoreUnexported(struct{}{}) //nolint:forbidigo // want nolint "nolint:forbidigo"
}
