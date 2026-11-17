package reads

import (
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

type accountsResponse struct {
	Accounts []account `json:"accounts"`
}

type account struct {
	AccountID string `json:"account_id"`
	Provider  string `json:"provider"`
}

func accountsType() schema.Type {
	return schema.Obj("Accounts", schema.F("accounts", schema.ArrayOf(schema.Obj("Account",
		schema.F("account_id", schema.Str()),
		schema.F("provider", schema.Str()),
	))))
}

// listAccounts is the one unscoped read, every account's identifier and provider, which the account
// selector and the entry redirect are built from (docs/UI.md section 17). It reads the accounts
// listing, which the application user reads in full and which holds nothing else (ADR-0091).
func (s *handlers) listAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := accounts.New(s.opts.Database).Accounts(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	out := accountsResponse{Accounts: []account{}}
	for _, row := range rows {
		out.Accounts = append(out.Accounts, account{AccountID: row.AccountID, Provider: row.AccountProvider})
	}
	server.WriteJSON(w, r, out)
}
