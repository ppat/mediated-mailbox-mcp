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

// TestNoFailFileIsSet fails unless the run has RAPID_NOFAILFILE=true, which the gating invocation sets
// (ADR-0069, row 6 of its ordinary-path table) and a demonstration therefore sets too.
func TestNoFailFileIsSet(t *testing.T) {
	if v := os.Getenv("RAPID_NOFAILFILE"); v != "true" {
		t.Fatalf("RAPID_NOFAILFILE is %q", v)
	}
}
