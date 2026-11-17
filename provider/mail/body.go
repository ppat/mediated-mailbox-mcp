package mail

import (
	"context"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Body returns the body of the message id names from m, the part it is released as. It asks for the
// rendered part first and, when the answer is that the provider holds that part apart, asks once more
// naming it, as core/mail's NextBodyPart decides, so a body takes at most two calls (ADR-0131). The
// worker's job kinds and the API server's body operation fetch every body through Body, so the rule
// has one home there.
func Body(ctx context.Context, m Mailbox, id string) (coremail.MessageBody, error) {
	var part coremail.BodyPart
	for {
		body, err := m.GetMessageBody(ctx, id, part)
		next, due := coremail.NextBodyPart(part, err)
		if !due {
			return body, err
		}
		part = next
	}
}
