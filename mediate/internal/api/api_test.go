package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// root returns the API root over reg.
func root(t *testing.T, reg service.Registry) http.Handler {
	t.Helper()
	h, err := api.Handler(reg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// answer is one response of the API root.
type answer struct {
	Status int
	Body   string
}

// send sends one request to h.
func send(t *testing.T, h http.Handler, method, target, body string) answer {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	got, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return answer{rec.Code, strings.TrimSpace(string(got))}
}

// The root serves each operation at its derived method and path, and rebuilds its argument object
// from the path, the query string typed by the schema, and the body. The operation receives the
// verified account and the rest of the object (ADR-0087).
func TestTheRootRebuildsEachOperationsArguments(t *testing.T) {
	reg, rec := fixtures(t)
	h := root(t, reg)
	cases := []struct {
		name, method, target, body string
		want                       call
	}{
		{
			"a read with every typed query parameter", "GET", "/api/accounts/acct-a/messages/m%201?limit=5&include_snippet=true&score=1.5&label=a%26b", "",
			call{"get_message", "acct-a", map[string]any{"message_id": "m 1", "limit": 5.0, "include_snippet": true, "score": 1.5, "label": "a&b"}},
		},
		{"a read with none", "GET", "/api/accounts/acct-a/messages/m1", "", call{"get_message", "acct-a", map[string]any{"message_id": "m1"}}},
		{"an account outside ASCII", "GET", "/api/accounts/%CE%A9mega%20%C3%BCnicode/messages/m1", "", call{"get_message", "Ωmega ünicode", map[string]any{"message_id": "m1"}}},
		{"an account holding a slash", "GET", "/api/accounts/a%2Fb/messages/m1", "", call{"get_message", "a/b", map[string]any{"message_id": "m1"}}},
		{"a sub-resource read", "GET", "/api/accounts/acct-a/reorg-plans/p1/sample?size=3", "", call{"sample_reorg_plan", "acct-a", map[string]any{"plan_id": "p1", "size": 3.0}}},
		{"the accounts listing", "GET", "/api/accounts?page_size=2", "", call{"list_accounts", "", map[string]any{"page_size": 2.0}}},
		{
			"a structured read", "POST", "/api/accounts/acct-a/messages:search", `{"query":{"from":["x"]},"cursor":"c"}`,
			call{"search_messages", "acct-a", map[string]any{"query": map[string]any{"from": []any{"x"}}, "cursor": "c"}},
		},
		{"a create", "POST", "/api/accounts/acct-a/labels", `{"name":"Receipts","dry_run":true}`, call{"create_label", "acct-a", map[string]any{"name": "Receipts", "dry_run": true}}},
		{"a reversible change", "POST", "/api/accounts/acct-a/messages:label", `{"message_ids":["m1"],"label":"L"}`, call{"label_messages", "acct-a", map[string]any{"message_ids": []any{"m1"}, "label": "L"}}},
		{"a disposal", "POST", "/api/accounts/acct-a/messages:trash", `{"message_ids":["m1"]}`, call{"trash_messages", "acct-a", map[string]any{"message_ids": []any{"m1"}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := send(t, h, c.method, c.target, c.body); got.Status != http.StatusOK {
				t.Fatalf("answered %d %s", got.Status, got.Body)
			}
			if diff := cmp.Diff([]call{c.want}, rec.take(), compare.Options); diff != "" {
				t.Errorf("calls (-want +got):\n%s", diff)
			}
		})
	}
}

// A request the root cannot turn into one argument object, or that names the account twice, never
// reaches an operation. HEAD is refused on every route, and a route serves only its derived method.
func TestTheRootRefusesWhatItCannotBind(t *testing.T) {
	reg, rec := fixtures(t)
	h := root(t, reg)
	get, post := "/api/accounts/acct-a/messages/m1", "/api/accounts/acct-a/messages:label"
	cases := []struct {
		name, method, target, body string
		status                     int
	}{
		{"HEAD on a read", "HEAD", get, "", 405},
		{"HEAD on the accounts listing", "HEAD", "/api/accounts", "", 405},
		{"a read's route posted to", "POST", get, `{}`, 405},
		{"a change's route read", "GET", post, "", 405},
		{"another method", "PUT", post, `{}`, 405},
		{"a path no operation takes", "GET", "/api/accounts/acct-a/things", "", 404},
		{"an operation's name as a path", "POST", "/api/get_message", `{}`, 404},
		{"a read with a body", "GET", get, `{"limit":1}`, 400},
		{"a query parameter the read does not declare", "GET", get + "?unknown=1", "", 400},
		{"a query parameter given twice", "GET", get + "?limit=1&limit=2", "", 400},
		{"an integer that is not one", "GET", get + "?limit=1.5", "", 400},
		{"a boolean written as a number", "GET", get + "?include_snippet=1", "", 400},
		{"a number that is not finite", "GET", get + "?score=NaN", "", 400},
		{"a path variable in the query string", "GET", "/api/accounts/acct-a/reorg-plans/p1/sample?plan_id=p2", "", 400},
		{"a query parameter on a change", "POST", post + "?label=L", `{"message_ids":[],"label":"L"}`, 400},
		{"a body that is not an object", "POST", post, `["m1"]`, 400},
		{"a body with bytes after the object", "POST", post, `{"label":"L"} {}`, 400},
		{"an account in the query string beside the path's", "GET", get + "?account_id=acct-a", "", 500},
		{"a case-variant account in the query string", "GET", get + "?Account_ID=Ωmega%20ünicode", "", 500},
		{"an account in the body beside the path's", "POST", post, `{"account_id":"acct-a","message_ids":[],"label":"L"}`, 500},
		{"a case-variant account in the body", "POST", post, `{"ACCOUNT_ID":"a/b","message_ids":[],"label":"L"}`, 500},
		{"an account the mediator does not serve", "GET", "/api/accounts/acct-z/messages/m1", "", 500},
		{"a declared argument in a change's query string", "POST", post + "?label=L", `{"message_ids":[]}`, 400},
		{"a body member given twice", "POST", post, `{"message_ids":[],"label":"x","label":"y"}`, 400},
		{"a case variant of a path variable in the body", "POST", "/api/accounts/acct-a/reorg-plans/P1/items:label", `{"label":"x","PLAN_ID":"P2"}`, 500},
		{"a case variant of a body member", "POST", post, `{"message_ids":[],"label":"x","LABEL":"y"}`, 500},
		{"an account in the accounts listing's query string", "GET", "/api/accounts?account_id=acct-a", "", 400},
		{"a case-variant account in the accounts listing's query string", "GET", "/api/accounts?ACCOUNT_ID=x", "", 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := send(t, h, c.method, c.target, c.body); got.Status != c.status {
				t.Errorf("answered %d %s, want %d", got.Status, got.Body, c.status)
			}
			if calls := rec.take(); len(calls) != 0 {
				t.Errorf("reached an operation: %+v", calls)
			}
		})
	}
	if got := send(t, h, "POST", "/api/accounts/acct-a/messages:fail", `{}`); got != (answer{500, `{"error":"the operation failed"}`}) {
		t.Errorf("a failing operation answered %+v", got)
	}
}

// Both roots generated from one registry carry exactly its operations, the API root's as the contract
// document's routes and as what it serves, the MCP root's as its tool list (ADR-0030, ADR-0053).
func TestBothRootsCarryExactlyTheRegistry(t *testing.T) {
	reg, _ := fixtures(t)
	var routes, names []string
	for _, op := range reg.Operations() {
		routes = append(routes, op.Method+" "+op.Path)
		names = append(names, op.Name)
	}
	slices.Sort(routes)

	apiRoot := root(t, reg)
	var documented []string
	for path, item := range document(t, reg).Paths.Map() {
		for method := range item.Operations() {
			documented = append(documented, method+" "+path)
			target := strings.NewReplacer("{account_id}", "acct-a", "{message_id}", "m1", "{plan_id}", "p1").Replace(path)
			if got := send(t, apiRoot, method, target, `{}`); got.Status == http.StatusNotFound || got.Status == http.StatusMethodNotAllowed {
				t.Errorf("the API root does not serve %s %s, which the contract document carries: %d", method, path, got.Status)
			}
		}
	}
	slices.Sort(documented)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	mcp.Handler(reg, "test").ServeHTTP(rec, req)
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("tools/list answered %q: %v", rec.Body, err)
	}
	var tools []string
	for _, tool := range listed.Result.Tools {
		tools = append(tools, tool.Name)
	}
	slices.Sort(tools)
	slices.Sort(names)

	want := map[string][]string{"routes": routes, "tools": names}
	got := map[string][]string{"routes": documented, "tools": tools}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("operations on each root (-want +got):\n%s", diff)
	}
}
