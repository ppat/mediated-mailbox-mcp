package gate_test

import (
	"os"
	"testing"
	"time"

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

// TestPausesWhenAsked lets a test of the runner act while the tests run. When $PAUSED_FILE is set, it
// creates that file and waits up to ten seconds for $RESUME_FILE to exist.
func TestPausesWhenAsked(t *testing.T) {
	paused, resume := os.Getenv("PAUSED_FILE"), os.Getenv("RESUME_FILE")
	if paused == "" {
		return
	}
	if err := os.WriteFile(paused, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 500 {
		if _, err := os.Stat(resume); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("never resumed")
}
