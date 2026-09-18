package baseline_test

import (
	"testing"

	"fixture/baseline"
)

func TestDoubleZero(t *testing.T) {
	if got := baseline.Double(0); got != 0 {
		t.Fatalf("Double(0) = %d, want 0", got)
	}
}

// TestUnrelatedFailure fails unpatched. The demonstration does not name it.
func TestUnrelatedFailure(t *testing.T) {
	t.Fatal("this test fails before any patch is applied")
}
