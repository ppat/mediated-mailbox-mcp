package gate_test

import (
	"os"
	"testing"

	"fixture/gate"
)

func TestRefusesFlagged(t *testing.T) {
	if gate.Allow(true) {
		t.Fatal("a flagged item passed")
	}
}

func TestAllowsClean(t *testing.T) {
	if !gate.Allow(false) {
		t.Fatal("a clean item was refused")
	}
}

// TestNoLeftover fails once a run has left a file named leftover in the package directory.
func TestNoLeftover(t *testing.T) {
	if _, err := os.Stat("leftover"); err == nil {
		t.Fatal("a file named leftover exists")
	}
}

// TestSkipped never runs to a pass, so a demonstration naming it must be refused.
func TestSkipped(t *testing.T) {
	t.Skip("skipped on purpose")
}
