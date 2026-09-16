//go:build banproof

//nolint:errcheck // reason // want suppression `names "errcheck"`
package jmap_test

import (
	"os"
	"testing"
)

// This file silences a control for the whole file, with the directive above the package clause.
func TestFileLevelDirective(t *testing.T) {
	_ = os.Remove(t.TempDir())
}
