package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// root returns the API root over reg.
func root(t *testing.T, reg service.Registry) http.Handler {
	t.Helper()
	h, err := api.Handler(reg, slog.New(slog.DiscardHandler))
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
		{"an account in the query string beside the path's", "GET", get + "?account_id=acct-a", "", 400},
		{"a case-variant account in the query string", "GET", get + "?Account_ID=Ωmega%20ünicode", "", 400},
		{"an account in the body beside the path's", "POST", post, `{"account_id":"acct-a","message_ids":[],"label":"L"}`, 400},
		{"a case-variant account in the body", "POST", post, `{"ACCOUNT_ID":"a/b","message_ids":[],"label":"L"}`, 400},
		{"an account the mediator does not serve", "GET", "/api/accounts/acct-z/messages/m1", "", 400},
		{"a declared argument in a change's query string", "POST", post + "?label=L", `{"message_ids":[]}`, 400},
		{"a body member given twice", "POST", post, `{"message_ids":[],"label":"x","label":"y"}`, 400},
		{"a case variant of a path variable in the body", "POST", "/api/accounts/acct-a/reorg-plans/P1/items:label", `{"label":"x","PLAN_ID":"P2"}`, 400},
		{"a case variant of a body member", "POST", post, `{"message_ids":[],"label":"x","LABEL":"y"}`, 400},
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
	// The root refuses an argument given twice itself, before the service layer's refusal of a
	// repeated key could, and names the argument.
	repeated := answer{400, `{"error":{"origin":"client","message":"the argument \"label\" is given more than once"}}`}
	if got := send(t, h, "POST", post, `{"message_ids":[],"label":"x","label":"y"}`); got != repeated {
		t.Errorf("a body member given twice answered %+v, want %+v", got, repeated)
	}
	if got := send(t, h, "POST", "/api/accounts/acct-a/messages:fail", `{}`); got != (answer{500, mediatorFailure}) {
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
	mcp.Handler(reg, "test", slog.New(slog.DiscardHandler)).ServeHTTP(rec, req)
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

// mediatorFailure is the content of a failure inside the mediator, written out here rather than read
// from the service layer.
const mediatorFailure = `{"error":{"origin":"mediator","message":"the operation failed inside the mediator; get_system_status reports the account's operational state"}}`

// Each failure names its origin, with the status the origin gives (ADR-0101). An operation's refusal
// of an argument and the registry's refusal of an account are the client's, a 400 carrying the
// message, a provider's failure is a 502 naming its kind and never the provider's text, and any other
// failure is a 500 that names no detail, so nothing internal leaks. The body is the content the MCP
// root carries for the same failure.
func TestEachFailureNamesItsOrigin(t *testing.T) {
	op := func(name string, err error) service.Operation {
		return service.Operation{
			Name: name, Description: "Fails.", Effect: service.Read, Path: "/api/accounts/{account_id}/" + name,
			Input:  json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
			Output: json.RawMessage(`{"type":"object"}`),
			Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) { return nil, err },
		}
	}
	reg, err := service.NewRegistry([]string{"acct-a"},
		op("refuses", service.Refuse("since must be an ISO 8601 timestamp in UTC ending in Z")),
		op("fails", errors.New("internal detail mmfieldmarker-leak")),
		op("upstream", service.FromProvider(fmt.Errorf("gmail: 503 mmfieldmarker-leak: %w", mail.ErrProvider))),
		op("throttled", service.FromProvider(fmt.Errorf("gmail: 429 mmfieldmarker-leak: %w", mail.ErrThrottled))),
		op("leasing", service.FromProvider(errors.New("leasing the call: mmfieldmarker-leak"))),
	)
	if err != nil {
		t.Fatal(err)
	}
	h := root(t, reg)
	var got []answer
	for _, path := range []string{"acct-a/refuses", "acct-a/fails", "acct-a/upstream", "acct-a/throttled", "acct-a/leasing", "acct-z/refuses"} {
		got = append(got, send(t, h, http.MethodGet, "/api/accounts/"+path, ""))
	}
	want := []answer{
		{http.StatusBadRequest, `{"error":{"origin":"client","message":"since must be an ISO 8601 timestamp in UTC ending in Z"}}`},
		{http.StatusInternalServerError, mediatorFailure},
		{http.StatusBadGateway, `{"error":{"origin":"provider","message":"the provider failed the request or did not answer"}}`},
		{http.StatusBadGateway, `{"error":{"origin":"provider","message":"the provider throttled the request; get_system_status reports the rate controller's state"}}`},
		{http.StatusInternalServerError, mediatorFailure},
		{http.StatusBadRequest, `{"error":{"origin":"client","message":"the operation's account_id names no account the mediator serves"}}`},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the answers (-want +got):\n%s", diff)
	}
}

// The API root refuses a query parameter named in another case than the served operation's schema
// declares, as the MCP root refuses the same argument (ADR-0087).
func TestAServedRouteTakesOnlyTheArgumentsItDeclares(t *testing.T) {
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operations(service.Sources{})...)
	if err != nil {
		t.Fatal(err)
	}
	h := root(t, reg)
	for _, target := range []string{
		"/api/accounts/acct-a/masking-events?SINCE=2026-07-21T20:00:00Z",
		"/api/accounts/acct-a/messages?Cursor=zz",
	} {
		if got := send(t, h, http.MethodGet, target, ""); got.Status != http.StatusBadRequest {
			t.Errorf("GET %s answered %d %s, want 400", target, got.Status, got.Body)
		}
	}
}

// Each failed call is logged at the level its origin gives, on both roots, through the logger the
// root is handed. A client's own request failing is routine, a provider's failure is one an operator
// may act on, and a failure inside the mediator is one an operator acts on (ADR-0119).
func TestEachFailureIsLoggedAtItsOriginsLevel(t *testing.T) {
	op := func(name string, err error) service.Operation {
		return service.Operation{
			Name: name, Description: "Fails.", Effect: service.Read, Path: "/api/accounts/{account_id}/" + name,
			Input:  json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
			Output: json.RawMessage(`{"type":"object"}`),
			Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) { return nil, err },
		}
	}
	reg, err := service.NewRegistry([]string{"acct-a"},
		op("refuses", service.Refuse("since must be an ISO 8601 timestamp in UTC ending in Z")),
		op("upstream", service.FromProvider(fmt.Errorf("gmail: 503: %w", mail.ErrProvider))),
		op("fails", errors.New("internal detail")),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"INFO refuses", "WARN upstream", "ERROR fails"}

	var apiLog strings.Builder
	h, err := api.Handler(reg, slog.New(slog.NewJSONHandler(&apiLog, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"refuses", "upstream", "fails"} {
		send(t, h, http.MethodGet, "/api/accounts/acct-a/"+name, "")
	}
	if diff := cmp.Diff(want, failures(t, apiLog.String()), compare.Options); diff != "" {
		t.Errorf("the API root's log (-want +got):\n%s", diff)
	}

	var mcpLog strings.Builder
	m := mcp.Handler(reg, "test", slog.New(slog.NewJSONHandler(&mcpLog, &slog.HandlerOptions{Level: slog.LevelDebug})))
	for i, name := range []string{"refuses", "upstream", "fails"} {
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":{"account_id":"acct-a"}}}`, i+1, name)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		m.ServeHTTP(httptest.NewRecorder(), req)
	}
	if diff := cmp.Diff(want, failures(t, mcpLog.String()), compare.Options); diff != "" {
		t.Errorf("the MCP root's log (-want +got):\n%s", diff)
	}
}

// failures returns the level and operation of each failed call log holds.
func failures(t *testing.T, log string) []string {
	t.Helper()
	var got []string
	for line := range strings.Lines(log) {
		var l struct{ Level, Msg, Operation string }
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			t.Fatalf("a log line is not a JSON object: %q: %v", line, err)
		}
		if l.Msg == "operation failed" {
			got = append(got, l.Level+" "+l.Operation)
		}
	}
	return got
}
