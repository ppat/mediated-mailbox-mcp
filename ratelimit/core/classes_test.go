package core_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// shares is a budget's split, class by class.
type shares struct {
	Interactive, Sync, Batch float64
}

func split(budget, target float64) shares {
	return shares{
		Interactive: core.Share(core.Interactive, budget, target),
		Sync:        core.Share(core.Sync, budget, target),
		Batch:       core.Share(core.Batch, budget, target),
	}
}

// ADR-0025's reservations as fractions of the target, with a cut taken from batch, then sync, then
// interactive. The target is 100 throughout.
func TestShare(t *testing.T) {
	cases := []struct {
		name   string
		budget float64
		want   shares
	}{
		{"at the target", 100, shares{30, 20, 50}},
		{"a halving comes out of batch alone", 50, shares{30, 20, 0}},
		{"a cut past batch comes out of sync", 40, shares{30, 10, 0}},
		{"a cut past sync comes out of interactive", 20, shares{20, 0, 0}},
		{"at the floor interactive has it all", 10, shares{10, 0, 0}},
		{"a small cut comes out of batch", 90, shares{30, 20, 40}},
		{"no budget", 0, shares{0, 0, 0}},
		{"a negative budget is none", -5, shares{0, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, split(c.budget, 100), compare.Options); diff != "" {
				t.Errorf("Share at budget %v (-want +got):\n%s", c.budget, diff)
			}
		})
	}
}

func TestAValueNoConstantNamesHasNoShare(t *testing.T) {
	for _, c := range []core.Class{0, core.Batch + 1, 255} {
		if got := core.Share(c, 100, 100); got != 0 {
			t.Errorf("Share(%d) = %v, want 0", c, got)
		}
	}
}
