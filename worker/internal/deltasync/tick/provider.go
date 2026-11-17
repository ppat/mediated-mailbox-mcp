package tick

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	providermail "github.com/ppat/mediated-mailbox-mcp/provider/mail"
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

// Through returns the Provider that calls the account through m, the mailbox of the tick's unit of
// work, which leases every call in its caller's class, the sync class for delta sync (ADR-0025).
func Through(m providermail.Mailbox) Provider { return through{m} }

// through is the Provider over one unit of work's mailbox.
type through struct{ providermail.Mailbox }

// GetMessageMetadata implements Provider. It splits ids into calls naming as many as fit one
// second's worth at the hard cap, as the mailbox says (ADR-0023).
func (t through) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	size := t.MetadataCallSize(len(ids))
	var out []mail.MessageMetadata
	for start := 0; start < len(ids); start += size {
		got, err := t.Mailbox.GetMessageMetadata(ctx, ids[start:min(start+size, len(ids))])
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// GetMessageBody implements Provider, a second call naming a part held apart (ADR-0131).
func (t through) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	return providermail.Body(ctx, t.Mailbox, id)
}
