package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"
)

// mailbox is one unit of work's mailbox, the one wrapper of every operation of the Provider Port.
// Each operation takes a lease sized by what the adapter's rate profile says it costs, at the
// account's target, waits for it no later than the unit's deadline, runs under the caller's timeout
// and tells the limiter how it went, through ratelimit.Leased. When the provider refuses the
// credential, the account is read again, and when it holds another credential the operation is made
// once more over a token made from it, a new lease included, and the later operations use that
// token (ADR-0090, ADR-0106).
type mailbox struct {
	build   func(*Token) (adapterMailbox, error)
	again   func(context.Context) (*Token, error)
	limiter *ratelimit.Limiter
	spend   ratelimit.Spend

	mu   sync.Mutex
	port adapterMailbox
}

var _ mail.Mailbox = (*mailbox)(nil)

// current returns the adapter the next operation calls.
func (m *mailbox) current() adapterMailbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.port
}

// call makes one operation of the kind op names through the adapter, as mailbox describes. A
// refused credential is read again once per operation, and the operation made again at most once.
func call[T any](ctx context.Context, m *mailbox, op coremail.ProviderOp, do func(context.Context, adapterMailbox) (T, error)) (T, error) {
	port := m.current()
	out, err := leased(ctx, m, port, op, do)
	if !errors.Is(err, coremail.ErrAuthentication) || m.again == nil {
		return out, err
	}
	token, aerr := m.again(ctx)
	if aerr != nil {
		return out, errors.Join(err, fmt.Errorf("reading the account again after its credential was refused: %w", aerr))
	}
	if token == nil {
		return out, err
	}
	port, berr := m.build(token)
	if berr != nil {
		return out, errors.Join(err, berr)
	}
	m.mu.Lock()
	m.port = port
	m.mu.Unlock()
	return leased(ctx, m, port, op, do)
}

// leased makes one operation through port under a lease of what port's rate profile says it costs.
func leased[T any](ctx context.Context, m *mailbox, port adapterMailbox, op coremail.ProviderOp, do func(context.Context, adapterMailbox) (T, error)) (T, error) {
	spend := m.spend
	spend.Cost = port.RateProfile().Cost(op)
	return ratelimit.Leased(ctx, m.limiter, spend, func(ctx context.Context) (T, error) { return do(ctx, port) })
}

// ListThreads implements mail.Mailbox.
func (m *mailbox) ListThreads(ctx context.Context, q coremail.Query, page coremail.PageToken) (coremail.Page[coremail.ThreadMetadata], error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpListThreads}, func(ctx context.Context, p adapterMailbox) (coremail.Page[coremail.ThreadMetadata], error) {
		return p.ListThreads(ctx, q, page)
	})
}

// GetThreadMetadata implements mail.Mailbox.
func (m *mailbox) GetThreadMetadata(ctx context.Context, threadID string) (coremail.ThreadMetadata, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpGetThreadMetadata}, func(ctx context.Context, p adapterMailbox) (coremail.ThreadMetadata, error) {
		return p.GetThreadMetadata(ctx, threadID)
	})
}

// GetMessageMetadata implements mail.Mailbox, priced for the number of identifiers ids names.
func (m *mailbox) GetMessageMetadata(ctx context.Context, ids []string) ([]coremail.MessageMetadata, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpGetMessageMetadata, Messages: len(ids)}, func(ctx context.Context, p adapterMailbox) ([]coremail.MessageMetadata, error) {
		return p.GetMessageMetadata(ctx, ids)
	})
}

// GetMessageBody implements mail.Mailbox. Each call is leased on its own, so a part held apart pays
// for its second call (ADR-0131).
func (m *mailbox) GetMessageBody(ctx context.Context, id string, part coremail.BodyPart) (coremail.MessageBody, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpGetMessageBody, Messages: 1}, func(ctx context.Context, p adapterMailbox) (coremail.MessageBody, error) {
		return p.GetMessageBody(ctx, id, part)
	})
}

// ListLabels implements mail.Mailbox.
func (m *mailbox) ListLabels(ctx context.Context) ([]coremail.Label, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpListLabels}, func(ctx context.Context, p adapterMailbox) ([]coremail.Label, error) {
		return p.ListLabels(ctx)
	})
}

// EnsureLabel implements mail.Mailbox.
func (m *mailbox) EnsureLabel(ctx context.Context, path string) (coremail.Label, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpEnsureLabel}, func(ctx context.Context, p adapterMailbox) (coremail.Label, error) {
		return p.EnsureLabel(ctx, path)
	})
}

// Mutate implements mail.Mailbox, priced for the number of ops it carries.
func (m *mailbox) Mutate(ctx context.Context, ops []coremail.MutationOp) (coremail.MutationResult, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpMutate, Messages: len(ops)}, func(ctx context.Context, p adapterMailbox) (coremail.MutationResult, error) {
		return p.Mutate(ctx, ops)
	})
}

// CurrentCursor implements mail.Mailbox.
func (m *mailbox) CurrentCursor(ctx context.Context) (coremail.Cursor, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpCurrentCursor}, func(ctx context.Context, p adapterMailbox) (coremail.Cursor, error) {
		return p.CurrentCursor(ctx)
	})
}

// ChangesSince implements mail.Mailbox.
func (m *mailbox) ChangesSince(ctx context.Context, cursor coremail.Cursor) (coremail.ChangeSet, error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpChangesSince}, func(ctx context.Context, p adapterMailbox) (coremail.ChangeSet, error) {
		return p.ChangesSince(ctx, cursor)
	})
}

// EnumerateAll implements mail.Mailbox.
func (m *mailbox) EnumerateAll(ctx context.Context, page coremail.PageToken) (coremail.Page[coremail.MessageMetadata], error) {
	return call(ctx, m, coremail.ProviderOp{Operation: coremail.OpEnumerateAll}, func(ctx context.Context, p adapterMailbox) (coremail.Page[coremail.MessageMetadata], error) {
		return p.EnumerateAll(ctx, page)
	})
}

// MetadataCallSize implements mail.Mailbox. It is the adapter's, which no lease covers.
func (m *mailbox) MetadataCallSize(most int) int {
	return m.current().MetadataCallSize(most)
}
