package lease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
)

// CallTimeout is how long a call made through Call has to answer once its lease is issued. A call
// that has not answered by then while its caller's context is still live is the provider's failure,
// so a provider that never answers cannot hold a job's run forever. It matches the mediator's default
// provider timeout, and its largest call, a page of an enumeration, sends six requests, each far
// shorter. It is a constant, not a setting, since no deployment has a reason of its own to tune it.
const CallTimeout = 30 * time.Second

// Call makes one call of port for the account under a lease in class, sized by what port's rate
// profile says op costs, and tells the limiter how the call went (ADR-0023, ADR-0024, ADR-0025).
// Every spending workload spends this way. A throttle and a server error the port reports go to the
// controller, and the throttle's backoff is waited out by the next lease. A refused body, a part held
// apart and every other answer that is not the provider's failure tell the controller nothing. A
// failure to tell the limiter fails the call, since a controller that never hears of a throttle
// would not slow down. The call has CallTimeout to answer.
func Call[T any](ctx context.Context, limiter *Limiter, port mail.Port[context.Context], account string, class core.Class, op mail.ProviderOp, call func(context.Context) (T, error)) (T, error) {
	var zero T
	cost := port.RateProfile().Cost(op)
	if _, err := limiter.Acquire(ctx, account, class, cost.Weight); err != nil {
		return zero, fmt.Errorf("leasing the call: %w", err)
	}
	start := time.Now()
	bounded, cancel := context.WithTimeout(ctx, CallTimeout)
	out, err := call(bounded)
	if err != nil && errors.Is(bounded.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		err = fmt.Errorf("%w: no answer within %s: %w", mail.ErrProvider, CallTimeout, err)
	}
	cancel()
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

// Body returns the message's body through port under leases in class, each call leased on its own
// at the cost the profile declares for it. The first call asks for the rendered part, and when it
// answers that the part is held apart, a second names it, as mail.NextBodyPart decides, so only a
// part held apart within the bound pays for two calls (ADR-0131).
func Body(ctx context.Context, limiter *Limiter, port mail.Port[context.Context], account string, class core.Class, id string) (mail.MessageBody, error) {
	var part mail.BodyPart
	for {
		body, err := Call(ctx, limiter, port, account, class, mail.ProviderOp{Operation: mail.OpGetMessageBody, Messages: 1}, func(ctx context.Context) (mail.MessageBody, error) {
			return port.GetMessageBody(ctx, id, part)
		})
		next, due := mail.NextBodyPart(part, err)
		if !due {
			return body, err
		}
		part = next
	}
}
