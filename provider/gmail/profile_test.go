package gmail_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// hardCap is one second's worth at the hard cap under Gmail's budget, 80% of 100 units (ADR-0024).
// The rate limiter refuses a call costing more.
const hardCap = 80

// Each call's declared cost is the sum of Gmail's published prices for the methods it calls, in its
// worst case, counting each read of the label table at one unit (ADR-0023).
func TestCostDeclaresEachCallsWorstCase(t *testing.T) {
	cases := []struct {
		op   mail.ProviderOp
		want mail.OpCost
	}{
		{mail.ProviderOp{Operation: mail.OpListThreads}, mail.OpCost{Weight: 52}},
		{mail.ProviderOp{Operation: mail.OpGetThreadMetadata}, mail.OpCost{Weight: 41}},
		{mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: 3}, mail.OpCost{Weight: 61, OpsCount: 3}},
		{mail.ProviderOp{Operation: mail.OpGetMessageMetadata}, mail.OpCost{}},
		{mail.ProviderOp{Operation: mail.OpGetMessageBody, Messages: 1}, mail.OpCost{Weight: 20, OpsCount: 1}},
		{mail.ProviderOp{Operation: mail.OpListLabels}, mail.OpCost{Weight: 1}},
		{mail.ProviderOp{Operation: mail.OpEnsureLabel}, mail.OpCost{Weight: 7}},
		{mail.ProviderOp{Operation: mail.OpMutate, Messages: 2}, mail.OpCost{Weight: 61, OpsCount: 2}},
		{mail.ProviderOp{Operation: mail.OpCurrentCursor}, mail.OpCost{Weight: 1}},
		{mail.ProviderOp{Operation: mail.OpChangesSince}, mail.OpCost{Weight: 3}},
		{mail.ProviderOp{Operation: mail.OpEnumerateAll}, mail.OpCost{Weight: 66}},
		{mail.ProviderOp{}, mail.OpCost{}},
		{mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: -1}, mail.OpCost{}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.op), func(t *testing.T) {
			if diff := cmp.Diff(c.want, gmail.Profile{}.Cost(c.op), compare.Options); diff != "" {
				t.Errorf("Cost (-want +got):\n%s", diff)
			}
		})
	}
}

// A call whose size depends on what it returns is sized by the adapter to fit one second's worth at
// the hard cap, so a listing is never refused. A call whose size the caller sets fits at its largest
// size and not one past it, so the caller splits its work there. At today's prices a metadata fetch
// holds three identifiers and a mutation two ops (ADR-0023).
func TestEveryPageFitsAndCallerSizedCallsSplitWhereTheSecondEnds(t *testing.T) {
	cost := func(op mail.Operation, messages int) float64 {
		return gmail.Profile{}.Cost(mail.ProviderOp{Operation: op, Messages: messages}).Weight
	}
	for _, op := range []mail.Operation{mail.OpListThreads, mail.OpEnumerateAll, mail.OpGetThreadMetadata} {
		if got := cost(op, 0); got > hardCap {
			t.Errorf("operation %d costs %v, past one second's worth at the hard cap, %d", op, got, hardCap)
		}
	}
	for op, largest := range map[mail.Operation]int{mail.OpGetMessageMetadata: 3, mail.OpMutate: 2} {
		if got := cost(op, largest); got > hardCap {
			t.Errorf("operation %d on %d messages costs %v, want it to fit %d", op, largest, got, hardCap)
		}
		if got := cost(op, largest+1); got <= hardCap {
			t.Errorf("operation %d on %d messages costs %v, want it past %d", op, largest+1, got, hardCap)
		}
	}
}

// The budget is Gmail's 6,000 units a minute per user per project, averaged over its minute.
func TestTheBudgetIsTheMinuteLimitPerSecond(t *testing.T) {
	if got := (gmail.Profile{}).BudgetPerSecond(); got != 100 {
		t.Errorf("BudgetPerSecond = %v, want 100", got)
	}
}

// A throttle's signal, its scope and retry-after included, is read from the port's throttle error
// however it is wrapped, and any other error is not a throttle.
func TestParseThrottle(t *testing.T) {
	signal := mail.ThrottleSignal{RetryAfterMillis: 2000, HasRetryAfter: true, Scope: mail.ScopePerProject}
	got, ok := gmail.Profile{}.ParseThrottle(fmt.Errorf("listing: %w", mail.ThrottleError{Signal: signal}))
	if !ok {
		t.Fatal("a wrapped throttle error is not read as a throttle")
	}
	if diff := cmp.Diff(signal, got, compare.Options); diff != "" {
		t.Errorf("signal (-want +got):\n%s", diff)
	}
	for _, err := range []error{mail.ErrProvider, mail.ErrThrottled, errors.New("gmail: 500")} {
		if _, ok := (gmail.Profile{}).ParseThrottle(err); ok {
			t.Errorf("%v is read as a throttle", err)
		}
	}
}

// Gmail's limits are fixed, so refreshing them does nothing and cannot fail.
func TestRefreshLimitsDoesNothing(t *testing.T) {
	if err := (gmail.Profile{}).RefreshLimits(context.Background()); err != nil {
		t.Errorf("RefreshLimits = %v, want nil", err)
	}
}
