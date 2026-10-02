package pass1

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// Leased returns the Fetch that asks port for a page of the account's enumeration under a lease in
// the batch class, as both of backfill's passes spend (ADR-0025).
func Leased(limiter *lease.Limiter, port mail.Port[context.Context], account string) Fetch {
	return func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return lease.Call(ctx, limiter, port, account, ratecore.Batch, mail.ProviderOp{Operation: mail.OpEnumerateAll}, func(ctx context.Context) (mail.Page[mail.MessageMetadata], error) {
			return port.EnumerateAll(ctx, token)
		})
	}
}
