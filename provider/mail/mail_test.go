package mail_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

const pkg = "github.com/ppat/mediated-mailbox-mcp/provider/mail"

// The body arrives only through the body fetch (ADR-0010). The port's form for mail has exactly
// these operations with exactly these signatures, and the call size, so an operation added to return
// the body in any form, a string included, fails, and so does a signature changed to return one. Of
// the types the operations return, which core/mail declares and its own tests pin field by field,
// only the body fetch's carries the body. The port carries no rate profile, which is the adapter's
// declaration and no operation of the port.
func TestOnlyTheBodyFetchReturnsABody(t *testing.T) {
	mustnotcompile.RequireMethods(t, pkg, "Mailbox",
		"ChangesSince func(ctx context.Context, cursor mail.Cursor) (mail.ChangeSet, error)",
		"CurrentCursor func(ctx context.Context) (mail.Cursor, error)",
		"EnsureLabel func(ctx context.Context, path string) (mail.Label, error)",
		"EnumerateAll func(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error)",
		"GetMessageBody func(ctx context.Context, id string, part mail.BodyPart) (mail.MessageBody, error)",
		"GetMessageMetadata func(ctx context.Context, ids []string) ([]mail.MessageMetadata, error)",
		"GetThreadMetadata func(ctx context.Context, threadID string) (mail.ThreadMetadata, error)",
		"ListLabels func(ctx context.Context) ([]mail.Label, error)",
		"ListThreads func(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error)",
		"MetadataCallSize func(most int) int",
		"Mutate func(ctx context.Context, ops []mail.MutationOp) (mail.MutationResult, error)")
}
