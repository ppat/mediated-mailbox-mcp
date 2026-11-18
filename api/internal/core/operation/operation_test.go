package operation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/operation"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// derived is what a test can observe of an effect's derivation.
type derived struct {
	Method      string
	Annotations operation.Annotations
	Derived     bool
}

// Each effect derives exactly the HTTP method and the four annotations ADR-0087's table gives it,
// written out here rather than read from the package, and an effect with no row derives nothing.
func TestDeriveGivesEachEffectItsRow(t *testing.T) {
	got := map[string]derived{}
	for name, e := range map[string]operation.Effect{
		"read": operation.Read, "structured read": operation.StructuredRead, "create": operation.Create,
		"reversible": operation.Reversible, "disposal": operation.Disposal, "none": 0, "past the table": operation.Disposal + 1,
	} {
		method, a, ok := operation.Derive(e)
		got[name] = derived{method, a, ok}
	}
	want := map[string]derived{
		"read":            {"GET", operation.Annotations{ReadOnly: true, Idempotent: true}, true},
		"structured read": {"POST", operation.Annotations{ReadOnly: true, Idempotent: true}, true},
		"create":          {"POST", operation.Annotations{}, true},
		"reversible":      {"POST", operation.Annotations{Idempotent: true}, true},
		"disposal":        {"POST", operation.Annotations{Destructive: true, Idempotent: true}, true},
		"none":            {},
		"past the table":  {},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// A path template's variables are its {name} segments, in order, and nothing else.
func TestPathVariables(t *testing.T) {
	got := operation.PathVariables("/api/accounts/{account_id}/messages/{message_id}/body")
	if diff := cmp.Diff([]string{"account_id", "message_id"}, got, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if got := operation.PathVariables("/api/accounts"); got != nil {
		t.Errorf("the accounts listing's path names %v", got)
	}
}

// accountInput is the input of an operation on one account that takes nothing else.
func accountInput() operation.Schema {
	return operation.Schema{Types: map[string]string{"account_id": "string"}, Required: []string{"account_id"}}
}

func whole(name string, e operation.Effect, path string) operation.Facts {
	return operation.Facts{Name: name, Description: "A fixture operation.", Effect: e, Path: path, Input: accountInput(), Handled: true}
}

// The generation checks decide from facts alone. A whole set of operations has no problem, and each
// fact that would leave an operation on one root, put it off the surface ADR-0087 describes, or let it
// express approval is named, by the operation's position and name, in the order the checks run.
func TestProblems(t *testing.T) {
	if got := operation.Problems([]operation.Facts{
		whole("list_accounts", operation.Read, "/api/accounts"),
		whole("get_thing", operation.Read, "/api/accounts/{account_id}/things"),
		whole("search_things", operation.StructuredRead, "/api/accounts/{account_id}/things:search"),
		whole("label_things", operation.Reversible, "/api/accounts/{account_id}/things:label"),
	}); got != nil {
		t.Errorf("a whole set of operations has the problems %q", got)
	}
	nameless := whole("Get Thing", operation.Read, "/api/accounts/{account_id}/things")
	nameless.Description, nameless.Handled = " ", false
	unread := whole("unread", operation.Read, "/api/accounts/{account_id}/unread")
	unread.Input = operation.Schema{Unreadable: true}
	wrongVar := whole("get_message", operation.Read, "/api/accounts/{account_id}/messages/{message_id}")
	wrongVar.Input.Types["filter"] = ""
	status := whole("archive_things", operation.Reversible, "/api/accounts/{account_id}/things:archive")
	status.Input.Status, status.Output = true, operation.Schema{Problem: "is missing", MCPHeader: true}
	got := operation.Problems([]operation.Facts{
		nameless,
		whole("approve_plan", operation.Create, "/api/accounts/{account_id}/plans:approve"),
		whole("get_thing", operation.Read, "/api/things"),
		whole("get_thing", operation.Read, "/api/accounts/{account_id}/things"),
		unread,
		wrongVar,
		status,
		{Name: "no_effect", Description: "A fixture operation.", Path: "/api/accounts/{account_id}/x", Input: accountInput(), Handled: true},
	})
	want := []string{
		`operation 0 ("Get Thing") has a name that is not lowercase letters, digits and underscores starting with a letter, at most 64 characters`,
		`operation 0 ("Get Thing") has no description`,
		`operation 0 ("Get Thing") has no handler`,
		`operation 1 ("approve_plan") has the path segment plans:approve, which names a plan's lifecycle`,
		`operation 1 ("approve_plan") has the name approve_plan, which names a plan's lifecycle`,
		`operation 1 ("approve_plan") is a read or a create whose path ends in a :verb route`,
		`operation 2 ("get_thing") has a path not under /api/accounts/{account_id}/, so it names no account in its path`,
		`operation 3 ("get_thing") repeats a name`,
		`operation 3 ("get_thing") repeats the route GET /api/accounts/{account_id}/things`,
		`operation 4 ("unread") has an input schema whose properties cannot be read`,
		`operation 5 ("get_message") has the path variable message_id, which is not a required string argument`,
		`operation 5 ("get_message") is a GET whose argument filter is not a scalar`,
		`operation 6 ("archive_things") changes the mailbox and declares a property named status in its input`,
		`operation 6 ("archive_things") has a schema carrying x-mcp-header`,
		`operation 6 ("archive_things") has an output schema that is missing`,
		`operation 7 ("no_effect") declares no effect the derivation table has a row for`,
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	badInput := whole("bad_input", operation.Read, "/api/accounts/{account_id}/y")
	badInput.Input = operation.Schema{Problem: `does not have the type "object"`, Types: map[string]string{"account_id": "string"}}
	if diff := cmp.Diff([]string{`operation 0 ("bad_input") has an input schema that does not have the type "object"`}, operation.Problems([]operation.Facts{badInput}), compare.Options); diff != "" {
		t.Errorf("an input that is not an object schema (-want +got):\n%s", diff)
	}
}
