package api

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
func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	ctx := r.Context()
	out := accountResponse{AccountID: account, OtherClients: []string{}}
	err := tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		return s.account(ctx, accountsetup.New(t), accountstate.New(t), ratestate.New(t), clientsetup.New(t), account, &out)
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

func (s *Server) account(ctx context.Context, q *accountsetup.Queries, state *accountstate.Queries, rates *ratestate.Queries,
	clients *clientsetup.Queries, account string, out *accountResponse,
) error {
	rows, err := q.AccountSetup(ctx, account)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("the account is not listed")
	}
	held := rows[0]
	out.Provider, out.OAuthClient, out.Connected = held.AccountProvider, optionalText(held.OauthClient), held.Connected
	out.Mailbox = optionalText(held.Mailbox)
	if held.LoweredTargetRate.Valid {
		f := widen(held.LoweredTargetRate.Float32)
		out.LoweredTarget = &f
	}
	progress, err := s.progress(ctx, state, account)
	if err != nil {
		return err
	}
	out.BackfillPass1Complete, out.BackfillPass2Complete = progress.Pass1Complete, progress.Pass2Complete
	out.SyncCursorAt, out.LastAuthAt, out.LastAuthOutcome = progress.SyncCursorAt, progress.LastAuthAt, progress.LastAuthOutcome
	rate, err := s.rate(ctx, rates, account)
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
func (s *Server) setTarget(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	var body targetRequest
	if !readBody(w, r, &body) {
		return
	}
	if setup.Target(body.LoweredTarget) != setup.None {
		writeFailure(w, r, clientFault(http.StatusBadRequest, string(setup.TargetRefused), "a lowered target is above 5% and at most 50% of the declared ceiling"))
		return
	}
	target := pgtype.Float4{}
	if body.LoweredTarget != nil {
		target = pgtype.Float4{Float32: float32(*body.LoweredTarget), Valid: true}
	}
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		n, err := accountsetup.New(t).SetLoweredTarget(r.Context(), accountsetup.SetLoweredTargetParams{LoweredTarget: target, AccountID: account})
		if err == nil && n == 0 {
			return errNotConnected
		}
		return err
	})
	if errors.Is(err, errNotConnected) {
		writeFailure(w, r, clientFault(http.StatusConflict, "not_connected", "the account is not connected, so it holds no target"))
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, targetAnswer(body))
}
