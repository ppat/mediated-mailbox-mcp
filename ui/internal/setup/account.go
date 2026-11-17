package setup

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	accountsetup "github.com/ppat/mediated-mailbox-mcp/db/accounts/setup"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	clientsetup "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/setup"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/setup"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// accountResponse is account settings' read, what the UI reads of the account's two rows and its rate
// state, never the credential (docs/UI.md sections 8.13 and 17.4, ADR-0084).
type accountResponse struct {
	AccountID             string   `json:"account_id"`
	Provider              string   `json:"provider"`
	OAuthClient           *string  `json:"oauth_client"`
	OtherClients          []string `json:"other_clients"`
	Mailbox               *string  `json:"mailbox"`
	Connected             bool     `json:"connected"`
	LastAuthAt            *string  `json:"last_auth_at"`
	LastAuthOutcome       *string  `json:"last_auth_outcome"`
	LoweredTarget         *float64 `json:"lowered_target"`
	CurrentTarget         *float64 `json:"current_target"`
	BackfillPass1Complete bool     `json:"backfill_pass1_complete"`
	BackfillPass2Complete bool     `json:"backfill_pass2_complete"`
	SyncCursorAt          *string  `json:"sync_cursor_at"`
}

func accountType() schema.Type {
	return schema.Obj("AccountSettings",
		schema.F("account_id", schema.Str()),
		schema.F("provider", schema.Str()),
		schema.F("oauth_client", schema.Null(schema.Str())),
		schema.F("other_clients", schema.ArrayOf(schema.Str())),
		schema.F("mailbox", schema.Null(schema.Str())),
		schema.F("connected", schema.Bool()),
		schema.F("last_auth_at", schema.Null(schema.Time())),
		schema.F("last_auth_outcome", schema.Null(schema.Str())),
		schema.F("lowered_target", schema.Null(schema.Num())),
		schema.F("current_target", schema.Null(schema.Num())),
		schema.F("backfill_pass1_complete", schema.Bool()),
		schema.F("backfill_pass2_complete", schema.Bool()),
		schema.F("sync_cursor_at", schema.Null(schema.Time())),
	)
}

// getAccount reads the path's account settings. The provider's other clients are the ones the
// account could move to (docs/UI.md section 8.13).
func (s *handlers) getAccount(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	ctx := r.Context()
	out := accountResponse{AccountID: account, OtherClients: []string{}}
	err := tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		return s.account(ctx, accountsetup.New(t), accountstate.New(t), ratestate.New(t), clientsetup.New(t), &out)
	})
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	server.WriteJSON(w, r, out)
}

func (s *handlers) account(ctx context.Context, q *accountsetup.Queries, state *accountstate.Queries, rates *ratestate.Queries,
	clients *clientsetup.Queries, out *accountResponse,
) error {
	rows, err := q.AccountSetup(ctx)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("the account is not listed")
	}
	held := rows[0]
	out.Provider, out.OAuthClient, out.Connected = held.AccountProvider, registry.OptionalText(held.OauthClient), held.Connected
	out.Mailbox = registry.OptionalText(held.Mailbox)
	if held.LoweredTargetRate.Valid {
		f := registry.Widen(held.LoweredTargetRate.Float32)
		out.LoweredTarget = &f
	}
	progress, err := registry.Progress(ctx, state)
	if err != nil {
		return err
	}
	out.BackfillPass1Complete, out.BackfillPass2Complete = progress.Pass1Complete, progress.Pass2Complete
	out.SyncCursorAt, out.LastAuthAt, out.LastAuthOutcome = progress.SyncCursorAt, progress.LastAuthAt, progress.LastAuthOutcome
	rate, err := registry.Rate(ctx, rates)
	if err != nil {
		return err
	}
	if rate != nil {
		out.CurrentTarget = &rate.Target
	}
	stored, err := clients.SetupClients(ctx)
	if err != nil {
		return err
	}
	for _, c := range stored {
		if c.Provider == held.AccountProvider && c.ClientName != held.OauthClient.String {
			out.OtherClients = append(out.OtherClients, c.ClientName)
		}
	}
	return nil
}

type targetRequest struct {
	LoweredTarget *float64 `json:"lowered_target"`
}

type targetAnswer struct {
	LoweredTarget *float64 `json:"lowered_target"`
}

func targetType() schema.Type {
	return schema.Obj("Target", schema.F("lowered_target", schema.Null(schema.Num())))
}

func targetAnswerType() schema.Type {
	return schema.Obj("TargetAnswer", schema.F("lowered_target", schema.Null(schema.Num())))
}

// errNotConnected is a target written for an account with no state row.
var errNotConnected = errors.New("the account has no state row")

// setTarget stores the account's lowered target, or clears it with null (docs/UI.md section 8.13).
// Each workload uses it from its next account reload.
func (s *handlers) setTarget(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	var body targetRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	if setup.Target(body.LoweredTarget) != setup.None {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, string(setup.TargetRefused), "a lowered target is above 5% and at most 50% of the declared ceiling"))
		return
	}
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		return storeTarget(r.Context(), accountsetup.New(t), body.LoweredTarget)
	})
	if errors.Is(err, errNotConnected) {
		server.WriteFailure(w, r, server.ClientFault(http.StatusConflict, "not_connected", "the account is not connected, so it holds no target"))
		return
	}
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	server.WriteJSON(w, r, targetAnswer(body))
}

// storeTarget writes the account's lowered target, or clears it when lowered is nil, through q, the
// statements of the transaction set to the account. An account with no state row holds no target, so
// its write is errNotConnected.
func storeTarget(ctx context.Context, q *accountsetup.Queries, lowered *float64) error {
	target := pgtype.Float4{}
	if lowered != nil {
		target = pgtype.Float4{Float32: float32(*lowered), Valid: true}
	}
	n, err := q.SetLoweredTarget(ctx, target)
	if err == nil && n == 0 {
		return errNotConnected
	}
	return err
}
