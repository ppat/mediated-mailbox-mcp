// Package bodies is what the API server's body operation needs of a provider, as two functions: the
// policy loaded fresh before a request decides, and a body fetched through a unit of work (ADR-0099,
// ADR-0130, ADR-0142). It is the one package of the API server that imports the connections and so
// the one that begins a unit of work, and the body operation alone is built with its functions. The
// service layer has the policy loaded first, before the gate decides, and begins a unit of work only
// once the gate has released the message, so a denial never reaches a provider (ADR-0002, ADR-0099).
package bodies

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
)

// leaseWait bounds how long one body request waits for its leases, counted from the request's
// arrival, so no request waits without limit whatever revision its client speaks. A request no lease
// comes for is answered within it, before Claude Code's own 60-second timer on a request's first
// response byte gives up on the call (ADR-0130).
const leaseWait = 45 * time.Second

// Bodies are the body operation's two functions over the process's Session and connections, with the
// body operation's own caller.
type Bodies struct {
	session *sessions.Session
	conns   *connections.Connections
	caller  *connections.Caller
	logger  *slog.Logger
}

// New returns the body operation's functions for the part p was handed to. Its caller is the body
// operation's own, built once, over the API server's pool, leasing every call in the interactive
// class, each call bounded by c's provider timeout, its series registered on p's registerer
// (ADR-0025, ADR-0101, ADR-0117).
func New(ctx context.Context, p process.Capabilities, c config.API) (*Bodies, error) {
	s, err := sessions.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	conns, err := connections.Of(ctx, p, c, s)
	if err != nil {
		return nil, err
	}
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	caller, err := conns.Caller(p, pool, provider.Options{Class: provider.Interactive, Timeout: time.Duration(c.ProviderTimeout)})
	if err != nil {
		return nil, err
	}
	return &Bodies{session: s, conns: conns, caller: caller, logger: p.Logger()}, nil
}

// Policy has the policy loaded by a load that starts after the call, and returns the account's
// policy from the policy then in force (ADR-0099). A load that fails leaves the policy in force,
// whose failure the policies alarm on (ADR-0041). A caller that stops waiting gets the policy that
// restricts every sender, so a decision is never made against a policy the load did not reach.
func (b *Bodies) Policy(ctx context.Context, account string) classify.Policy {
	if err := b.session.FreshPolicy(ctx); err != nil {
		if ctx.Err() != nil {
			return classify.Policy{}
		}
		b.logger.Error("the policy was not loaded, so the active policy decides", "error", err)
	}
	return b.session.Policy(account)
}

// Open begins a body request's unit of work on the account, through the body operation's caller,
// whose leases wait no later than 45 seconds after the request arrived (ADR-0130). It refuses an
// account the Session's list in force does not hold or that has no connection.
func (b *Bodies) Open(_ context.Context, account string, arrived time.Time) (service.Provider, error) {
	work, err := b.conns.Begin(account, b.caller, arrived.Add(leaseWait))
	if err != nil {
		return nil, err
	}
	return &unit{work: work, account: account, logger: b.logger}, nil
}

// unit is one body request's unit of work, whose every call goes through its mailbox.
type unit struct {
	work    *connections.Work
	account string
	logger  *slog.Logger
}

// Body returns the message's body, in a second call naming the rendered part when the first answers
// that it is held apart (ADR-0131).
func (u *unit) Body(ctx context.Context, id string) (coremail.MessageBody, error) {
	return mail.Body(ctx, u.work.Mailbox(), id)
}

// Metadata returns the message's metadata, or an error wrapping mail.ErrNotFound when the provider
// no longer holds the message.
func (u *unit) Metadata(ctx context.Context, id string) (coremail.MessageMetadata, error) {
	read, err := u.work.Mailbox().GetMessageMetadata(ctx, []string{id})
	if err != nil {
		return coremail.MessageMetadata{}, err
	}
	if len(read) == 0 {
		return coremail.MessageMetadata{}, fmt.Errorf("message %s: %w", id, coremail.ErrNotFound)
	}
	return read[0], nil
}

// Done ends the unit of work, which writes a rotated credential back and records the latest
// authentication attempt (ADR-0082, ADR-0089, ADR-0097). A failure of either is logged apart and fails
// nothing, since the request's answer is already decided. The connections keep a rotated credential
// whose write-back failed, and the next unit writes it back (ADR-0090).
func (u *unit) Done(ctx context.Context) {
	writeBack, recorded := u.work.End(ctx)
	if writeBack != nil {
		u.logger.Error("handing the account's credential over failed", "account", u.account, "error", writeBack)
	}
	if recorded != nil {
		u.logger.Error("recording the last authentication attempt failed", "account", u.account, "error", recorded)
	}
}
