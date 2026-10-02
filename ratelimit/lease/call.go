package lease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
)

// Call makes one call of port for the account under a lease in class, sized by what port's rate
// profile says op costs, and tells the limiter how the call went (ADR-0023, ADR-0024, ADR-0025).
// Every spending workload spends this way. A throttle and a server error the port reports go to the
// controller, and the throttle's backoff is waited out by the next lease. A failure to tell the
// limiter fails the call, since a controller that never hears of a throttle would not slow down.
func Call[T any](ctx context.Context, limiter *Limiter, port mail.Port[context.Context], account string, class core.Class, op mail.ProviderOp, call func(context.Context) (T, error)) (T, error) {
	var zero T
	cost := port.RateProfile().Cost(op)
	if _, err := limiter.Acquire(ctx, account, class, cost.Weight); err != nil {
		return zero, fmt.Errorf("leasing the call: %w", err)
	}
	start := time.Now()
	out, err := call(ctx)
	var told error
	switch signal, throttled := mail.Throttled(err); {
	case err == nil:
		told = limiter.Succeeded(ctx, account, cost, time.Since(start))
	case throttled:
		told = limiter.Throttled(ctx, account, signal)
	case errors.Is(err, mail.ErrProvider):
		told = limiter.ServerErrored(ctx, account)
	}
	if told != nil {
		return zero, errors.Join(err, fmt.Errorf("telling the rate limiter how the call went: %w", told))
	}
	return out, err
}
