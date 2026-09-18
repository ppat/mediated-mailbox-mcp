package count_test

import (
	"os"
	"testing"
)

// TestSeedIsSet fails unless the run has a non-zero RAPID_SEED, which the runner provides when the
// environment does not.
func TestSeedIsSet(t *testing.T) {
	if seed := os.Getenv("RAPID_SEED"); seed == "" || seed == "0" {
		t.Fatalf("RAPID_SEED is %q", seed)
	}
}
