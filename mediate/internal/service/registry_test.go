package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func handle(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// operation returns an operation named name that generates onto both roots.
func operation(name string) service.Operation {
	return service.Operation{
		Name:        name,
		Description: "Returns nothing.",
		Input:       json.RawMessage(`{"type": "object",  "properties": {"account_id": {"type": "string"}}, "required": ["account_id"]}`),
		Output:      json.RawMessage(`{"type":"object"}`),
		Handle:      handle,
	}
}

// accountInput is operation's input schema in its compact form.
const accountInput = `{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`

// entry is what a test can observe of one operation in a registry.
type entry struct {
	Name, Description, Input, Output string
}

func observe(r service.Registry) []entry {
	out := []entry{}
	for _, op := range r.Operations() {
		out = append(out, entry{op.Name, op.Description, string(op.Input), string(op.Output)})
	}
	return out
}

// Operations that carry everything both roots need generate, sorted by name, each found by its name,
// and each schema in one compact form so both roots and the contract document carry the same bytes.
func TestWholeOperationsGenerate(t *testing.T) {
	reg, err := service.NewRegistry(nil, operation("list_labels"), operation("get_message"))
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{"get_message", "Returns nothing.", accountInput, `{"type":"object"}`},
		{"list_labels", "Returns nothing.", accountInput, `{"type":"object"}`},
	}
	if diff := cmp.Diff(want, observe(reg), compare.Options); diff != "" {
		t.Errorf("registry (-want +got):\n%s", diff)
	}
	for _, name := range []string{"get_message", "list_labels"} {
		if op, ok := reg.Lookup(name); !ok || op.Name != name {
			t.Errorf("Lookup(%q) = %q, %v", name, op.Name, ok)
		}
	}
	if _, ok := reg.Lookup("send_message"); ok {
		t.Error("Lookup found an operation the registry does not hold")
	}
	var zero service.Registry
	if got := zero.Operations(); len(got) != 0 {
		t.Errorf("the zero registry holds %d operations", len(got))
	}
}

// An operation missing anything one root needs would generate onto the other root alone, so it fails
// generation, and the registry that comes back holds nothing (ADR-0053).
func TestAOneSidedEntryFailsGeneration(t *testing.T) {
	cases := []struct {
		name   string
		ops    func() []service.Operation
		reason string
	}{
		{"no name", func() []service.Operation { return []service.Operation{operation("")} }, "has a name that is not"},
		{"a name no tool can carry", func() []service.Operation { return []service.Operation{operation("get-message")} }, "has a name that is not"},
		{"a name starting with a digit", func() []service.Operation { return []service.Operation{operation("1get")} }, "has a name that is not"},
		{"a name too long for a tool", func() []service.Operation { return []service.Operation{operation(strings.Repeat("a", 65))} }, "has a name that is not"},
		{"a name repeated", func() []service.Operation { return []service.Operation{operation("get"), operation("get")} }, "repeats a name"},
		{"no description", func() []service.Operation {
			op := operation("get")
			op.Description = " "
			return []service.Operation{op}
		}, "has no description"},
		{"no input schema", func() []service.Operation {
			op := operation("get")
			op.Input = nil
			return []service.Operation{op}
		}, "has an input schema that is missing"},
		{"an input schema that is not JSON", func() []service.Operation {
			op := operation("get")
			op.Input = json.RawMessage(`{"type":`)
			return []service.Operation{op}
		}, "has an input schema that is not a JSON object"},
		{"an input schema that is not an object's", func() []service.Operation {
			op := operation("get")
			op.Input = json.RawMessage(`{"type":"string"}`)
			return []service.Operation{op}
		}, `has an input schema that does not have the type "object"`},
		{"no output schema", func() []service.Operation {
			op := operation("get")
			op.Output = nil
			return []service.Operation{op}
		}, "has an output schema that is missing"},
		{"an output schema that is not an object's", func() []service.Operation {
			op := operation("get")
			op.Output = json.RawMessage(`[]`)
			return []service.Operation{op}
		}, "has an output schema that is not a JSON object"},
		{"an input schema without account_id", func() []service.Operation {
			op := operation("get")
			op.Input = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)
			return []service.Operation{op}
		}, "has an input schema that does not require the string account_id"},
		{"account_id declared but optional", func() []service.Operation {
			op := operation("get")
			op.Input = json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}}}`)
			return []service.Operation{op}
		}, "has an input schema that does not require the string account_id"},
		{"account_id declared as a number", func() []service.Operation {
			op := operation("get")
			op.Input = json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"integer"}},"required":["account_id"]}`)
			return []service.Operation{op}
		}, "has an input schema that does not require the string account_id"},
		{"no handler", func() []service.Operation {
			op := operation("get")
			op.Handle = nil
			return []service.Operation{op}
		}, "has no handler"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ops := append([]service.Operation{operation("whole")}, c.ops()...)
			reg, err := service.NewRegistry(nil, ops...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Errorf("NewRegistry error = %v, want one saying %q", err, c.reason)
			}
			if got := reg.Operations(); len(got) != 0 {
				t.Errorf("a registry that failed generation holds %d operations", len(got))
			}
		})
	}
}

// Approval is not in the client surface's vocabulary (ADR-0020, ADR-0030). No operation the mediator
// serves names, describes or takes an approval, so neither root has one to generate.
func TestTheSurfaceHasNoApprovalVocabulary(t *testing.T) {
	reg, err := service.NewRegistry(nil, service.Operations()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range reg.Operations() {
		for _, text := range []string{op.Name, op.Description, string(op.Input), string(op.Output)} {
			if strings.Contains(strings.ToLower(text), "approv") {
				t.Errorf("operation %s carries approval in its vocabulary: %s", op.Name, text)
			}
		}
	}
}

// Every operation but the accounts listing is refused before it runs when its account_id is missing,
// names no account the mediator serves, or is named more than once, including under a key differing
// only in case (ADR-0087). An operation that runs gets the verified account and its arguments without
// account_id, so it has no other account to read. The accounts listing takes no account, and its
// schema need not declare one.
func TestTheServiceLayerRefusesAMissingOrUnknownAccount(t *testing.T) {
	type run struct{ Op, Account, Input string }
	var ran []run
	record := func(name string) func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return func(_ context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
			ran = append(ran, run{name, account, string(input)})
			return json.RawMessage(`{}`), nil
		}
	}
	scopedOp := operation("get_thing")
	scopedOp.Handle = record("get_thing")
	listing := service.Operation{
		Name: "list_accounts", Description: "Lists the accounts.",
		Input: json.RawMessage(`{"type":"object"}`), Output: json.RawMessage(`{"type":"object"}`),
		Handle: record("list_accounts"),
	}
	reg, err := service.NewRegistry([]string{"acct-a", "acct-b"}, scopedOp, listing)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, op, input string
		want            error
		ran             []run
	}{
		{"a served account", "get_thing", `{"id":"m1","account_id":"acct-b","n":2}`, nil, []run{{"get_thing", "acct-b", `{"id":"m1","n":2}`}}},
		{"a served account and nothing else", "get_thing", `{"account_id":"acct-a"}`, nil, []run{{"get_thing", "acct-a", `{}`}}},
		{"no account_id", "get_thing", `{}`, service.ErrMissingAccount, nil},
		{"a null account_id", "get_thing", `{"account_id":null}`, service.ErrMissingAccount, nil},
		{"an account_id that is not a string", "get_thing", `{"account_id":7}`, service.ErrMissingAccount, nil},
		{"arguments that are not an object", "get_thing", `["account_id","acct-a"]`, service.ErrMissingAccount, nil},
		{"an account the mediator does not serve", "get_thing", `{"account_id":"acct-c"}`, service.ErrUnknownAccount, nil},
		{"an empty account_id", "get_thing", `{"account_id":""}`, service.ErrUnknownAccount, nil},
		{"an account differing only in case", "get_thing", `{"account_id":"ACCT-A"}`, service.ErrUnknownAccount, nil},
		{"account_id twice", "get_thing", `{"account_id":"acct-c","account_id":"acct-a"}`, service.ErrAmbiguousAccount, nil},
		{"a case-variant key beside account_id", "get_thing", `{"account_id":"acct-c","Account_ID":"acct-a"}`, service.ErrAmbiguousAccount, nil},
		{"a case-variant key alone", "get_thing", `{"ACCOUNT_ID":"acct-a"}`, service.ErrAmbiguousAccount, nil},
		{"an operation the registry does not hold", "approve_plan", `{"account_id":"acct-a"}`, service.ErrNoOperation, nil},
		{"the accounts listing with no account", "list_accounts", `{}`, nil, []run{{"list_accounts", "", `{}`}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ran = nil
			_, err := reg.Call(t.Context(), c.op, json.RawMessage(c.input))
			if !errors.Is(err, c.want) {
				t.Errorf("error %v, want %v", err, c.want)
			}
			if diff := cmp.Diff(c.ran, ran, compare.Options); diff != "" {
				t.Errorf("operations run (-want +got):\n%s", diff)
			}
		})
	}
}
