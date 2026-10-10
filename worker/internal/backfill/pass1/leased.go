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

// LeasedMetadata returns the Metadata that asks port for the metadata of a call's messages under a
// lease in the batch class, priced for the number of identifiers the call names (ADR-0025, ADR-0120).
func LeasedMetadata(limiter *lease.Limiter, port mail.Port[context.Context], account string) Metadata {
	return func(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
		op := mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: len(ids)}
		return lease.Call(ctx, limiter, port, account, ratecore.Batch, op, func(ctx context.Context) ([]mail.MessageMetadata, error) {
			return port.GetMessageMetadata(ctx, ids)
		})
	}
}
