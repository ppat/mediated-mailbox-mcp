package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"
)

// Spend is what one leased call spends, for whom and how long it may take.
type Spend struct {
	// Account is the account the call is for.
	Account string
	// Class is the priority class the lease is asked in (ADR-0025).
	Class core.Class
	// Cost is what the call costs, as its provider's rate profile declares it (ADR-0023).
	Cost mail.OpCost
	// WaitUntil is the latest the lease is waited for, zero for no time of its own. A wait cut off by
	// it while the caller's context is live fails the call as the caller's own, not the provider's
	// (ADR-0130).
	WaitUntil time.Time
	// Timeout is how long the call has to answer once its lease is issued. A call that has not
	// answered by then while the caller's context is live is the provider's failure (ADR-0101).
	Timeout time.Duration
}

// Leased makes one call under a lease of s.Cost in s.Class for s.Account, and tells the limiter how
// the call went (ADR-0023, ADR-0024, ADR-0025). It takes a cost and not a mailbox, so the provider
// library's one wrapper of a mailbox's operations spends through it for every operation. A throttle
// and a server error the call reports go to the controller, and the throttle's backoff is waited out
// by the next lease. A refused body, a part held apart and every other answer that is not the
// provider's failure tell the controller nothing. A failure to tell the limiter fails the call, since
// a controller that never hears of a throttle would not slow down.
func Leased[T any](ctx context.Context, limiter *Limiter, s Spend, call func(context.Context) (T, error)) (T, error) {
	var zero T
	waiting, stopWaiting := ctx, context.CancelFunc(func() {})
	if !s.WaitUntil.IsZero() {
		waiting, stopWaiting = context.WithDeadline(ctx, s.WaitUntil)
	}
	_, err := limiter.Acquire(waiting, s.Account, s.Class, s.Cost.Weight)
	cutOff := errors.Is(waiting.Err(), context.DeadlineExceeded) && ctx.Err() == nil
	stopWaiting()
	if err != nil && cutOff {
		return zero, fmt.Errorf("leasing the call: no lease by the deadline for its leases: %w", err)
	}
	if err != nil {
		return zero, fmt.Errorf("leasing the call: %w", err)
	}
	start := time.Now()
	bounded, cancel := context.WithTimeout(ctx, s.Timeout)
	out, err := call(bounded)
	if err != nil && errors.Is(bounded.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		err = fmt.Errorf("%w: no answer within %s: %w", mail.ErrProvider, s.Timeout, err)
	}
	cancel()
	var told error
	switch signal, throttled := mail.Throttled(err); {
	case err == nil:
		told = limiter.Succeeded(ctx, s.Account, s.Cost, time.Since(start))
	case throttled:
		told = limiter.Throttled(ctx, s.Account, signal)
	case errors.Is(err, mail.ErrProvider):
		told = limiter.ServerErrored(ctx, s.Account)
	}
	if told != nil {
		return zero, errors.Join(err, fmt.Errorf("telling the rate limiter how the call went: %w", told))
	}
	return out, err
}
