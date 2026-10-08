package tick

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
)

// Provider is what a tick asks of the provider for one account. Its errors wrap the Provider Port's.
type Provider interface {
	CurrentCursor(ctx context.Context) (mail.Cursor, error)
	ChangesSince(ctx context.Context, cursor mail.Cursor) (mail.ChangeSet, error)
	// GetMessageMetadata asks for the metadata of ids in calls that fit, so it takes any number.
	GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error)
	ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error)
	GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error)
}

// Leased is the Provider that calls port for the account under a lease in the sync class, each call
// sized by what port's rate profile says it costs (ADR-0025).
type Leased struct {
	Limiter *lease.Limiter
	Port    mail.Port[context.Context]
	Account string
}

var _ Provider = Leased{}

func call[T any](ctx context.Context, l Leased, op mail.ProviderOp, do func(context.Context) (T, error)) (T, error) {
	return lease.Call(ctx, l.Limiter, l.Port, l.Account, ratecore.Sync, op, do)
}

// CurrentCursor implements Provider.
func (l Leased) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return call(ctx, l, mail.ProviderOp{Operation: mail.OpCurrentCursor}, l.Port.CurrentCursor)
}

// ChangesSince implements Provider.
func (l Leased) ChangesSince(ctx context.Context, cursor mail.Cursor) (mail.ChangeSet, error) {
	return call(ctx, l, mail.ProviderOp{Operation: mail.OpChangesSince}, func(ctx context.Context) (mail.ChangeSet, error) {
		return l.Port.ChangesSince(ctx, cursor)
	})
}

// GetMessageMetadata implements Provider. It splits ids into calls naming as many as fit one second's
// worth at the hard cap (ADR-0023).
func (l Leased) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	profile := l.Port.RateProfile()
	size := mail.CallSize(func(n int) float64 {
		return profile.Cost(mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: n}).Weight
	}, profile.BudgetPerSecond(), len(ids))
	var out []mail.MessageMetadata
	for start := 0; start < len(ids); start += size {
		chunk := ids[start:min(start+size, len(ids))]
		got, err := call(ctx, l, mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: len(chunk)}, func(ctx context.Context) ([]mail.MessageMetadata, error) {
			return l.Port.GetMessageMetadata(ctx, chunk)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// ListThreads implements Provider.
func (l Leased) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	return call(ctx, l, mail.ProviderOp{Operation: mail.OpListThreads}, func(ctx context.Context) (mail.Page[mail.ThreadMetadata], error) {
		return l.Port.ListThreads(ctx, q, page)
	})
}

// GetMessageBody implements Provider.
func (l Leased) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	return call(ctx, l, mail.ProviderOp{Operation: mail.OpGetMessageBody, Messages: 1}, func(ctx context.Context) (mail.MessageBody, error) {
		return l.Port.GetMessageBody(ctx, id)
	})
}
