package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// A served operation takes its arguments only under the names its input schema declares, in exactly
// that case, and never as null, as the API root takes them, so the MCP root carries no call the API
// root refuses (ADR-0087). The refusal comes before the operation reads anything.
func TestAServedOperationTakesOnlyTheArgumentsItDeclares(t *testing.T) {
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operations(service.Sources{DB: unreachable{}, Accounts: func() []service.Account { return nil }})...)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		op, args, want string
	}{
		{"list_masking_events", `{"account_id":"acct-a","SINCE":"2026-07-21T20:00:00+02:00"}`, "the arguments name one the operation does not take"},
		{"list_masking_events", `{"account_id":"acct-a","Since":"2026-07-21T20:00:00Z"}`, "the arguments name one the operation does not take"},
		{"list_messages", `{"account_id":"acct-a","Cursor":"zz"}`, "the arguments name one the operation does not take"},
		{"get_message", `{"account_id":"acct-a","Message_ID":"m-1"}`, "the arguments name one the operation does not take"},
		{"get_system_status", `{"account_id":"acct-a","bogus":1}`, "the arguments name one the operation does not take"},
		{"list_accounts", `{"Page":1}`, "the arguments name one the operation does not take"},
		{"list_masking_events", `{"account_id":"acct-a","since":null}`, "since may not be null"},
		{"list_threads", `{"account_id":"acct-a","cursor":null}`, "cursor may not be null"},
		{"get_thread", `{"account_id":"acct-a","thread_id":null}`, "thread_id may not be null"},
	}
	for _, c := range cases {
		_, err := reg.Call(t.Context(), c.op, json.RawMessage(c.args))
		var refused *service.ArgumentError
		if !errors.As(err, &refused) || refused.Error() != c.want {
			t.Errorf("%s %s: %v, want the refusal %q", c.op, c.args, err, c.want)
		}
	}
}

// unreachable is a database no transaction can begin on, so a call the refusal lets through fails
// rather than reading anything.
type unreachable struct{}

func (unreachable) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("the test reaches no database")
}
