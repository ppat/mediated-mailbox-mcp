package pass1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// Leased returns the Fetch that asks port for a page of the account's enumeration under a lease in
// the batch class, sized by what port's rate profile says the call costs, and tells the limiter how
// the call went (ADR-0023, ADR-0024, ADR-0025). A throttle and a server error the port reports go to
// the controller, and the throttle's backoff is waited out by the next lease. A failure to tell the
// limiter fails the page, since a controller that never hears of a throttle would not slow down.
func Leased(limiter *lease.Limiter, port mail.Port[context.Context], account string) Fetch {
	return func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		cost := port.RateProfile().Cost(mail.ProviderOp{Operation: mail.OpEnumerateAll})
		if _, err := limiter.Acquire(ctx, account, ratecore.Batch, cost.Weight); err != nil {
			return mail.Page[mail.MessageMetadata]{}, fmt.Errorf("leasing the call: %w", err)
		}
		start := time.Now()
		page, err := port.EnumerateAll(ctx, token)
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
			return mail.Page[mail.MessageMetadata]{}, errors.Join(err, fmt.Errorf("telling the rate limiter how the call went: %w", told))
		}
		return page, err
	}
}
