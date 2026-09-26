package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// served are the accounts the fixture registry serves. One holds letters outside ASCII and a space,
// and one a slash, so a path segment carrying either is exercised.
var served = []string{"acct-a", "Ωmega ünicode", "a/b"}

// call is what reached an operation, the account the registry verified and the arguments without
// account_id.
type call struct {
	Op      string
	Account string
	Input   any
}

// recorder keeps every call that reached a fixture operation.
type recorder struct {
	mu    sync.Mutex
	calls []call
}

func (r *recorder) take() []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.calls
	r.calls = nil
	return out
}

// fixtureOps are one operation of each effect class, the accounts listing and a read of a
// sub-resource, shaped as ADR-0087 shapes them. Each echoes the account and the arguments it received.
func fixtureOps(rec *recorder) []service.Operation {
	echo := func(name string) func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return func(_ context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
			var v any
			if err := json.Unmarshal(input, &v); err != nil {
				return nil, err
			}
			rec.mu.Lock()
			rec.calls = append(rec.calls, call{name, account, v})
			rec.mu.Unlock()
			return json.Marshal(map[string]any{"account": account, "input": v})
		}
	}
	op := func(name string, effect service.Effect, path, input string) service.Operation {
		return service.Operation{
			Name: name, Description: "A fixture " + name + ".", Effect: effect, Path: path,
			Input: json.RawMessage(input), Output: json.RawMessage(`{"type":"object","properties":{"account":{"type":"string"}}}`),
			Handle: echo(name),
		}
	}
	return []service.Operation{
		op("list_accounts", service.Read, "/api/accounts",
			`{"type":"object","properties":{"page_size":{"type":"integer"}}}`),
		op("get_message", service.Read, "/api/accounts/{account_id}/messages/{message_id}",
			`{"type":"object","properties":{"account_id":{"type":"string"},"message_id":{"type":"string"},"include_snippet":{"type":"boolean"},"score":{"type":"number"},"label":{"type":"string"},"limit":{"type":"integer"}},"required":["account_id","message_id"]}`),
		op("sample_reorg_plan", service.Read, "/api/accounts/{account_id}/reorg-plans/{plan_id}/sample",
			`{"type":"object","properties":{"account_id":{"type":"string"},"plan_id":{"type":"string"},"size":{"type":"integer"}},"required":["account_id","plan_id"]}`),
		op("search_messages", service.StructuredRead, "/api/accounts/{account_id}/messages:search",
			`{"type":"object","properties":{"account_id":{"type":"string"},"query":{"type":"object"},"cursor":{"type":"string"}},"required":["account_id","query"]}`),
		op("create_label", service.Create, "/api/accounts/{account_id}/labels",
			`{"type":"object","properties":{"account_id":{"type":"string"},"name":{"type":"string"},"dry_run":{"type":"boolean"}},"required":["account_id","name"]}`),
		op("label_messages", service.Reversible, "/api/accounts/{account_id}/messages:label",
			`{"type":"object","properties":{"account_id":{"type":"string"},"message_ids":{"type":"array","items":{"type":"string"}},"label":{"type":"string"},"dry_run":{"type":"boolean"}},"required":["account_id","message_ids","label"]}`),
		op("trash_messages", service.Disposal, "/api/accounts/{account_id}/messages:trash",
			`{"type":"object","properties":{"account_id":{"type":"string"},"message_ids":{"type":"array","items":{"type":"string"}},"dry_run":{"type":"boolean"}},"required":["account_id","message_ids"]}`),
		func() service.Operation {
			o := op("fail_messages", service.Reversible, "/api/accounts/{account_id}/messages:fail",
				`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`)
			o.Handle = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return nil, errors.New("internal detail mmfieldmarker-leak")
			}
			return o
		}(),
	}
}

// fixtures returns the fixture registry over the served accounts and the recorder of its calls.
func fixtures(t testing.TB) (service.Registry, *recorder) {
	t.Helper()
	rec := &recorder{}
	reg, err := service.NewRegistry(served, fixtureOps(rec)...)
	if err != nil {
		t.Fatal(err)
	}
	return reg, rec
}
