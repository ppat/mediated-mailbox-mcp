//go:build integration

package lease_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// An account given a lower target spends under it, from its first ask on, and grows no higher however
// many calls succeed, while another account of the same Limiter keeps the default target. The hard cap
// stays where the ceiling puts it for both (ADR-0024).
func TestALoweredTargetBoundsOnlyItsOwnAccount(t *testing.T) {
	conn := superuser(t)
	lowered, other := newAccount(t, conn), newAccount(t, conn)
	l, err := lease.NewWithTargets(spenders(t), ceiling, map[string]float64{lowered: 0.2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{lowered, other} {
		for range 30 {
			if _, err := l.Acquire(t.Context(), account, core.Batch, 1); err != nil {
				t.Fatal(err)
			}
			if err := l.Succeeded(t.Context(), account, mail.OpCost{Weight: 1, OpsCount: 1}, time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, c := range []struct {
		account string
		target  float64
	}{{lowered, 20}, {other, 50}} {
		var rate, target, hard float64
		err := conn.QueryRow(t.Context(), "SELECT current_rate, target_rate, hard_cap FROM rate_state WHERE account_id = $1", c.account).
			Scan(&rate, &target, &hard)
		if err != nil {
			t.Fatal(err)
		}
		if rate > c.target || target != c.target || hard != hardCap {
			t.Errorf("account with target %v: rate %v, target %v, hard cap %v, want the rate at most %v, the target %v and the hard cap %v",
				c.target, rate, target, hard, c.target, c.target, hardCap)
		}
	}
}

// A stored target above half the ceiling, at the floor or below it refuses the whole Limiter and names
// the account, so no account runs at a target other than the one its state row sets (ADR-0024).
func TestAnOutOfRangeTargetRefusesTheLimiter(t *testing.T) {
	for _, target := range []float64{0.51, 0.05, 0.01, 0} {
		_, err := lease.NewWithTargets(nil, ceiling, map[string]float64{"the-account": target}, nil)
		if !errors.Is(err, core.ErrTargetOutOfRange) || !strings.Contains(err.Error(), `"the-account"`) {
			t.Errorf("NewWithTargets with target %v returned %v, want ErrTargetOutOfRange naming the account", target, err)
		}
	}
}
