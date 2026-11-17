package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// servicePackage is the service layer's import path, which the shape checks load.
const servicePackage = "github.com/ppat/mediated-mailbox-mcp/api/internal/service"

// Only the body operation is handed Bodies, whose Open begins the unit of work every provider call
// goes through, and nothing else in the service layer holds the Provider that Open returns (ADR-0002,
// ADR-0030). The checks read the declarations' types, not the handlers' code. Every other
// operation's handler is a method of Sources, whose fields are pinned here. So are the fields of
// Account, which the accounts listing's function returns, since function types are not followed and
// Account is the one type of this package a field's function returns. The takers of Bodies are
// pinned to Operations, the body operation's builder, and the handler with its two helpers, so a
// field, parameter, receiver or package-level variable carrying Bodies anywhere else adds a taker.
// No declaration may carry a Provider, so a type the sources carry gaining a method that returns
// one, such as the pool's interface, adds takers too. What the takers' own code does with Bodies or
// a value got from it, a value of a function or interface type that carries a way to a provider at
// run time, such as a package-level variable holding b.Open or a type assertion on the pool to a
// type that also opens a unit of work, and a type the sources carry gaining a method that returns
// an interface declared outside the service layer that reaches a provider, are held by review.
func TestOnlyTheBodyOperationIsHandedAWayToAProvider(t *testing.T) {
	mustnotcompile.RequireFields(t, servicePackage, "Sources",
		"DB Database", "Accounts func() []Account", "Policy func(account string) classify.Policy", "Now func() time.Time")
	mustnotcompile.RequireFields(t, servicePackage, "Account", "ID string", "Provider string")
	mustnotcompile.RequireTakers(t, servicePackage, "Bodies",
		"Operations", "bodyOperation", "bodySources.audit", "bodySources.getMessageBody", "bodySources.markdown")
	mustnotcompile.RequireTakers(t, servicePackage, "Provider")
}

// A served operation takes its arguments only under the names its input schema declares, in exactly
// that case, and never as null, as the API root takes them, so the MCP root carries no call the API
// root refuses (ADR-0087). The refusal comes before the operation reads anything.
func TestAServedOperationTakesOnlyTheArgumentsItDeclares(t *testing.T) {
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operations(service.Sources{DB: unreachable{}, Accounts: func() []service.Account { return nil }}, service.Bodies{})...)
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

func (unreachable) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("the test reaches no database")
}
