package ratelimit

import (
	"context"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"
)

// CallTimeout is how long a call made through Call has to answer once its lease is issued. A call
// that has not answered by then while its caller's context is still live is the provider's failure,
// so a provider that never answers cannot hold a job's run forever. It matches the API server's default
// provider timeout, and its largest call, a page of an enumeration, sends six requests, each far
// shorter. It is a constant, not a setting, since no deployment has a reason of its own to tune it.
const CallTimeout = 30 * time.Second

// Call makes one call of port for the account through Leased, under a lease in class sized by what
// port's rate profile says op costs, with no deadline for the lease of its own and CallTimeout to
// answer, and so tells the limiter how the call went as Leased does (ADR-0023, ADR-0024, ADR-0025).
func Call[T any](ctx context.Context, limiter *Limiter, port mail.Port[context.Context], account string, class core.Class, op mail.ProviderOp, call func(context.Context) (T, error)) (T, error) {
	return Leased(ctx, limiter, Spend{Account: account, Class: class, Cost: port.RateProfile().Cost(op), Timeout: CallTimeout}, call)
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
