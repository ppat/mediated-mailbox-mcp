package service_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// op returns an operation with the given effect and path whose input declares account_id and the
// other named properties, each a required string unless given a schema.
func op(name string, effect service.Effect, path, input string) service.Operation {
	return service.Operation{
		Name:        name,
		Description: "A fixture operation.",
		Effect:      effect,
		Path:        path,
		Input:       json.RawMessage(input),
		Output:      json.RawMessage(`{"type":"object"}`),
		Handle:      handle,
	}
}

const accountOnly = `{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`

// derived is what a test can observe of an operation's derivation.
type derived struct {
	Method                                       string
	ReadOnly, Destructive, Idempotent, OpenWorld bool
}

// Each effect derives exactly the HTTP method and the four annotations ADR-0087's table gives it,
// and nothing on an operation declares either (ADR-0087).
func TestEachEffectDerivesItsRow(t *testing.T) {
	reg, err := service.NewRegistry(nil,
		op("get_thing", service.Read, "/api/accounts/{account_id}/things", accountOnly),
		op("search_things", service.StructuredRead, "/api/accounts/{account_id}/things:search", accountOnly),
		op("create_thing", service.Create, "/api/accounts/{account_id}/things", accountOnly),
		op("label_things", service.Reversible, "/api/accounts/{account_id}/things:label", accountOnly),
		op("trash_things", service.Disposal, "/api/accounts/{account_id}/things:trash", accountOnly),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]derived{}
	for _, d := range reg.Operations() {
		a := d.Annotations
		got[d.Name] = derived{d.Method, a.ReadOnly, a.Destructive, a.Idempotent, a.OpenWorld}
	}
	want := map[string]derived{
		"get_thing":     {"GET", true, false, true, false},
		"search_things": {"POST", true, false, true, false},
		"create_thing":  {"POST", false, false, false, false},
		"label_things":  {"POST", false, false, true, false},
		"trash_things":  {"POST", false, true, true, false},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("derivations (-want +got):\n%s", diff)
	}
}

// An operation the surface of ADR-0087 cannot carry, or that could express approval, fails
// generation with a reason naming what is wrong, and the registry that comes back holds nothing.
func TestTheSurfaceRefusesWhatItCannotCarry(t *testing.T) {
	under := "/api/accounts/{account_id}/"
	with := func(props string, required ...string) string {
		req, err := json.Marshal(append([]string{"account_id"}, required...))
		if err != nil {
			t.Fatal(err)
		}
		return `{"type":"object","properties":{"account_id":{"type":"string"}` + props + `},"required":` + string(req) + `}`
	}
	cases := []struct {
		name   string
		op     service.Operation
		reason string
	}{
		{"no effect", op("get_thing", 0, under+"things", accountOnly), "declares no effect the derivation table has a row for"},
		{"an effect with no row", op("get_thing", 99, under+"things", accountOnly), "declares no effect the derivation table has a row for"},
		{"a path outside the account", op("get_thing", service.Read, "/api/things", accountOnly), "has a path not under /api/accounts/{account_id}/"},
		{"the accounts path for a write", op("create_account", service.Create, "/api/accounts", `{"type":"object"}`), "takes the accounts listing's path without being a read"},
		{"a path variable not in the schema", op("get_thing", service.Read, under+"things/{thing_id}", accountOnly), "has the path variable thing_id, which is not a required string argument"},
		{"a path variable that is optional", op("get_thing", service.Read, under+"things/{thing_id}", with(`,"thing_id":{"type":"string"}`)), "has the path variable thing_id, which is not a required string argument"},
		{"a path variable that is not a string", op("get_thing", service.Read, under+"things/{thing_id}", with(`,"thing_id":{"type":"integer"}`, "thing_id")), "has the path variable thing_id, which is not a required string argument"},
		{"a path variable named twice", op("get_thing", service.Read, under+"things/{thing_id}/{thing_id}", with(`,"thing_id":{"type":"string"}`, "thing_id")), "names the path variable thing_id twice"},
		{"a GET taking an object", op("get_thing", service.Read, under+"things", with(`,"filter":{"type":"object"}`)), "is a GET whose argument filter is not a scalar"},
		{"a GET taking an array", op("get_thing", service.Read, under+"things", with(`,"ids":{"type":"array","items":{"type":"string"}}`)), "is a GET whose argument ids is not a scalar"},
		{"a GET taking an untyped argument", op("get_thing", service.Read, under+"things", with(`,"any":{}`)), "is a GET whose argument any is not a scalar"},
		{"a structured read not on a :search route", op("search_things", service.StructuredRead, under+"things", accountOnly), "is a structured read whose path does not end in a :search route"},
		{"a change not on a :verb route", op("label_things", service.Reversible, under+"things", accountOnly), "is a change whose path does not end in a :verb route other than :search"},
		{"a change on the :search route", op("trash_things", service.Disposal, under+"things:search", accountOnly), "is a change whose path does not end in a :verb route other than :search"},
		{"a read on a :verb route", op("get_thing", service.Read, under+"things:label", accountOnly), "is a read or a create whose path ends in a :verb route"},
		{"a create on a :verb route", op("create_thing", service.Create, under+"things:make", accountOnly), "is a read or a create whose path ends in a :verb route"},
		{"a :verb segment before the end", op("get_thing", service.Read, under+"things:label/{thing_id}", with(`,"thing_id":{"type":"string"}`, "thing_id")), "has the :verb segment things:label before the path's end"},
		{"a segment that is not a name", op("get_thing", service.Read, under+"Things", accountOnly), "has the path segment Things"},
		{"status on a create", op("create_reorg_plan", service.Create, under+"reorg-plans", with(`,"status":{"type":"string"}`)), "changes the mailbox and declares a property named status in its input"},
		{"status in another case on a reversible change", op("revise_reorg", service.Reversible, under+"reorgs/{id}:revise", with(`,"id":{"type":"string"},"Status":{"type":"string"}`, "id")), "changes the mailbox and declares a property named status in its input"},
		{"status nested in an object on a disposal", op("trash_things", service.Disposal, under+"things:trash", with(`,"changes":{"type":"object","properties":{"STATUS":{"type":"string"}}}`)), "changes the mailbox and declares a property named status in its input"},
		{"status nested in an array's items on a change", op("label_things", service.Reversible, under+"things:label", with(`,"items":{"type":"array","items":{"type":"object","properties":{"status":{"type":"string"}}}}`)), "changes the mailbox and declares a property named status in its input"},
		{"a name approving", op("approve_reorg_plan", service.Reversible, under+"reorg-plans:mark", accountOnly), "has the name approve_reorg_plan, which names a plan's lifecycle"},
		{"a name applying", op("apply_labels", service.Reversible, under+"labels:set", accountOnly), "has the name apply_labels, which names a plan's lifecycle"},
		{"a name holding approved", op("mark_approved", service.Reversible, under+"marks:set", accountOnly), "has the name mark_approved, which names a plan's lifecycle"},
		{"a name holding approval", op("list_plan_approval", service.Read, under+"plans", accountOnly), "has the name list_plan_approval, which names a plan's lifecycle"},
		{"a name holding roll_back", op("roll_back_plan", service.Reversible, under+"plans:revert", accountOnly), "has the name roll_back_plan, which names a plan's lifecycle"},
		{"a :verb approving", op("mark_reorg_plan", service.Reversible, under+"reorg-plans:approve", accountOnly), "has the path segment reorg-plans:approve, which names a plan's lifecycle"},
		{"a :verb holding approve", op("mark_reorg_plan", service.Reversible, under+"reorg-plans:approve-all", accountOnly), "has the path segment reorg-plans:approve-all, which names a plan's lifecycle"},
		{"a segment holding approve", op("get_thing", service.Read, under+"approve-plan", accountOnly), "has the path segment approve-plan, which names a plan's lifecycle"},
		{"a segment rolling back", op("get_thing", service.Read, under+"rollback", accountOnly), "has the path segment rollback, which names a plan's lifecycle"},
		{"a segment holding roll-back", op("get_thing", service.Read, under+"plan-roll-back", accountOnly), "has the path segment plan-roll-back, which names a plan's lifecycle"},
		{"a segment applying", op("get_thing", service.Read, under+"applied-labels", accountOnly), "has the path segment applied-labels, which names a plan's lifecycle"},
		{"x-mcp-header in the input", op("get_thing", service.Read, under+"things", `{"type":"object","properties":{"account_id":{"type":"string","x-mcp-header":"Account"}},"required":["account_id"]}`), "has a schema carrying x-mcp-header"},
		{"x-mcp-header in the output", func() service.Operation {
			o := op("get_thing", service.Read, under+"things", accountOnly)
			o.Output = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","X-MCP-Header":"Id"}}}`)
			return o
		}(), "has a schema carrying x-mcp-header"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg, err := service.NewRegistry(nil, c.op)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Errorf("NewRegistry error = %v, want one saying %q", err, c.reason)
			}
			if got := reg.Operations(); len(got) != 0 {
				t.Errorf("a registry that failed generation holds %d operations", len(got))
			}
		})
	}
}

// A read may take status as a filter, as listing plans by their state does, and a structured read
// may take it in its query. Only an operation that changes the mailbox is refused one (ADR-0087).
func TestAReadMayFilterOnStatus(t *testing.T) {
	_, err := service.NewRegistry(nil,
		op("list_reorg_plans", service.Read, "/api/accounts/{account_id}/reorg-plans", `{"type":"object","properties":{"account_id":{"type":"string"},"status":{"type":"string"}},"required":["account_id"]}`),
		op("search_reorg_plans", service.StructuredRead, "/api/accounts/{account_id}/reorg-plans:search", `{"type":"object","properties":{"account_id":{"type":"string"},"query":{"type":"object","properties":{"status":{"type":"string"}}}},"required":["account_id"]}`),
	)
	if err != nil {
		t.Errorf("a read filtering on status failed generation: %v", err)
	}
}

// Two operations on one route fail generation, since a root could serve only one of them.
func TestARepeatedRouteFailsGeneration(t *testing.T) {
	_, err := service.NewRegistry(nil,
		op("get_thing", service.Read, "/api/accounts/{account_id}/things", accountOnly),
		op("list_things", service.Read, "/api/accounts/{account_id}/things", accountOnly),
	)
	if err == nil || !strings.Contains(err.Error(), "repeats the route GET /api/accounts/{account_id}/things") {
		t.Errorf("NewRegistry error = %v, want the repeated route refused", err)
	}
}
