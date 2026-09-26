package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// accountInput is the input schema of an operation taking only its account.
const accountInput = `{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`

// empty is a handler returning an empty result.
func empty(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// registry holds, for the account acct-a, an operation of each effect class. The read echoes its
// arguments, and a reversible change always fails with an error whose text must never reach the
// client.
func registry(t *testing.T) service.Registry {
	t.Helper()
	under := "/api/accounts/{account_id}/"
	simple := func(name string, effect service.Effect, path string) service.Operation {
		return service.Operation{
			Name: name, Description: "A fixture " + name + ".", Effect: effect, Path: under + path,
			Input: json.RawMessage(accountInput), Output: json.RawMessage(`{"type":"object"}`), Handle: empty,
		}
	}
	reg, err := service.NewRegistry([]string{"acct-a"},
		simple("search_things", service.StructuredRead, "things:search"),
		simple("create_thing", service.Create, "things"),
		simple("trash_things", service.Disposal, "things:trash"),
		service.Operation{
			Name:        "echo",
			Description: "Returns its arguments.",
			Effect:      service.Read,
			Path:        under + "echo",
			Input:       json.RawMessage(accountInput),
			Output:      json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}}}`),
			Handle: func(_ context.Context, account string, in json.RawMessage) (json.RawMessage, error) {
				return json.Marshal(map[string]any{"account": account, "input": in})
			},
		},
		service.Operation{
			Name:        "fail",
			Description: "Always fails.",
			Effect:      service.Reversible,
			Path:        under + "things:fail",
			Input:       json.RawMessage(accountInput),
			Output:      json.RawMessage(`{"type":"object"}`),
			Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return nil, errors.New("internal detail mmfieldmarker-leak")
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// era is one protocol revision a client speaks.
type era struct {
	name string
	// modern marks 2026-07-28, which carries the revision in each request's metadata and headers.
	modern bool
}

var (
	legacy = era{name: "2025-06-18"}
	modern = era{name: "2026-07-28", modern: true}
)

// response is a JSON-RPC response and the HTTP status it came with.
type response struct {
	Status int
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// call sends one JSON-RPC request to the MCP root over its HTTP transport as a client of e does,
// with no session. toolName is the tool a tools/call names, which 2026-07-28 repeats in a header.
func call(t *testing.T, h http.Handler, e era, method, toolName string, params map[string]any) response {
	t.Helper()
	req := request(t, e, method, toolName, params)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	r := response{Status: rec.Code}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("%s at %s answered %d %q: %v", method, e.name, rec.Code, rec.Body, err)
	}
	return r
}

func request(t *testing.T, e era, method, toolName string, params map[string]any) *http.Request {
	t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	if e.modern {
		params["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    e.name,
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "test", "version": "0"},
		}
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if e.modern {
		req.Header.Set("MCP-Protocol-Version", e.name)
		req.Header.Set("Mcp-Method", method)
		if toolName != "" {
			req.Header.Set("Mcp-Name", toolName)
		}
	}
	return req
}

// asJSON compares JSON values rather than their bytes.
var asJSON = cmp.Transformer("json", func(m json.RawMessage) any {
	var v any
	if err := json.Unmarshal(m, &v); err != nil {
		return string(m)
	}
	return v
})

// tool is what a client sees of one tool in the tool list.
type tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Annotations  string          `json:"-"`
}

// UnmarshalJSON keeps the annotations' bytes as the client received them, so the test pins them
// literally.
func (t *tool) UnmarshalJSON(b []byte) error {
	type plain tool
	var raw struct {
		plain
		Annotations json.RawMessage `json:"annotations"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*t = tool(raw.plain)
	t.Annotations = string(raw.Annotations)
	return nil
}

// The annotations each effect class derives, as the listing carries them, all four hints present
// (ADR-0087, ADR-0086).
const (
	readAnnotations           = `{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`
	createAnnotations         = `{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`
	reversibleAnnotations     = `{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`
	disposalAnnotations       = `{"destructiveHint":true,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`
	objectOutput              = `{"type":"object"}`
	structuredReadAnnotations = readAnnotations
)

// The tool list is the registry's operations, each with its name, description, both schemas and the
// four annotations its effect class derives, literally and with every hint present, at either
// revision (ADR-0053, ADR-0087, ADR-0086).
func TestTheToolListIsTheRegistry(t *testing.T) {
	h := mcp.Handler(registry(t), "test")
	fixture := func(name, annotations string) tool {
		return tool{Name: name, Description: "A fixture " + name + ".", InputSchema: json.RawMessage(accountInput), OutputSchema: json.RawMessage(objectOutput), Annotations: annotations}
	}
	want := []tool{
		fixture("create_thing", createAnnotations),
		{
			Name: "echo", Description: "Returns its arguments.",
			InputSchema:  json.RawMessage(accountInput),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}}}`),
			Annotations:  readAnnotations,
		},
		{Name: "fail", Description: "Always fails.", InputSchema: json.RawMessage(accountInput), OutputSchema: json.RawMessage(objectOutput), Annotations: reversibleAnnotations},
		fixture("search_things", structuredReadAnnotations),
		fixture("trash_things", disposalAnnotations),
	}
	for _, e := range []era{legacy, modern} {
		var got struct {
			Tools []tool `json:"tools"`
		}
		if err := json.Unmarshal(call(t, h, e, "tools/list", "", nil).Result, &got); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got.Tools, compare.Options, asJSON); diff != "" {
			t.Errorf("tools at %s (-want +got):\n%s", e.name, diff)
		}
	}
}

// The root offers tools and nothing else (ADR-0030, ADR-0086). It advertises exactly the tools
// capability at initialization before 2026-07-28 and at discovery from it, a subscription stream
// closes at once rather than being held open, and a request for prompts, resources or a log level
// is a method the server does not have, carried in a 200 before 2026-07-28 and a 404 from it.
func TestTheRootOffersToolsOnly(t *testing.T) {
	h := mcp.Handler(registry(t), "test")
	capabilities := func(r response) json.RawMessage {
		var got struct {
			Capabilities json.RawMessage `json:"capabilities"`
		}
		if err := json.Unmarshal(r.Result, &got); err != nil {
			t.Fatal(err)
		}
		return got.Capabilities
	}
	advertised := map[string]json.RawMessage{
		"initialize": capabilities(call(t, h, legacy, "initialize", "", map[string]any{
			"protocolVersion": legacy.name,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1"},
		})),
		"server/discover": capabilities(call(t, h, modern, "server/discover", "", nil)),
	}
	want := map[string]json.RawMessage{"initialize": json.RawMessage(`{"tools":{}}`), "server/discover": json.RawMessage(`{"tools":{}}`)}
	if diff := cmp.Diff(want, advertised, compare.Options, asJSON); diff != "" {
		t.Errorf("advertised capabilities (-want +got):\n%s", diff)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	listen := request(t, modern, "subscriptions/listen", "", map[string]any{"notifications": map[string]any{"toolsListChanged": true}}).WithContext(ctx)
	start := time.Now()
	h.ServeHTTP(httptest.NewRecorder(), listen)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("subscriptions/listen held its stream open for %s", elapsed)
	}

	for _, e := range []era{legacy, modern} {
		wantStatus := http.StatusOK
		if e.modern {
			wantStatus = http.StatusNotFound
		}
		// 2026-07-28 repeats the prompt's name and the resource's URI in a header.
		named := map[string]string{"prompts/get": "x", "resources/read": "file:///x"}
		for _, method := range []string{"prompts/list", "prompts/get", "resources/list", "resources/read", "resources/templates/list", "resources/subscribe", "logging/setLevel"} {
			r := call(t, h, e, method, named[method], map[string]any{"name": "x", "uri": "file:///x", "level": "debug"})
			if r.Status != wantStatus || r.Error == nil || r.Error.Code != -32601 || r.Result != nil {
				t.Errorf("%s at %s answered %d with %s, want %d and error -32601", method, e.name, r.Status, r.Result, wantStatus)
			}
		}
	}

	// Two requests a check reading the body apart from the SDK would read differently. One names a
	// second method under a key differing only in case, and one is a batch, which 2025-03-26 allows.
	crafted := []struct {
		name, protocol, body string
		answers              int
	}{
		{"a second method under a case-variant key", "", `{"jsonrpc":"2.0","id":1,"method":"prompts/list","METHOD":"tools/list","params":{}}`, 1},
		{"a batch at 2025-03-26", "2025-03-26", `[{"jsonrpc":"2.0","id":1,"method":"prompts/list","params":{}},{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}]`, 2},
	}
	for _, c := range crafted {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", bytes.NewReader([]byte(c.body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if c.protocol != "" {
			req.Header.Set("MCP-Protocol-Version", c.protocol)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		body := bytes.TrimSpace(rec.Body.Bytes())
		var answers []response
		if len(body) > 0 && body[0] == '[' {
			if err := json.Unmarshal(body, &answers); err != nil {
				t.Fatalf("%s answered %q: %v", c.name, body, err)
			}
		} else {
			var one response
			if err := json.Unmarshal(body, &one); err != nil {
				t.Fatalf("%s answered %d %q: %v", c.name, rec.Code, body, err)
			}
			answers = append(answers, one)
		}
		if len(answers) != c.answers {
			t.Errorf("%s answered %d %s, want %d answers", c.name, rec.Code, body, c.answers)
		}
		for _, a := range answers {
			if a.Error == nil || a.Error.Code != -32601 || a.Result != nil {
				t.Errorf("%s answered %d %s, want every request refused with -32601", c.name, rec.Code, body)
			}
		}
	}
}

// A tool call runs the registry's operation on the call's arguments and returns its result, at either
// revision. A failed operation, a missing account among them, is a tool error whose structured
// content and text say only that it failed.
func TestAToolCallRunsTheOperation(t *testing.T) {
	h := mcp.Handler(registry(t), "test")
	for _, e := range []era{legacy, modern} {
		type result struct {
			IsError           bool            `json:"isError"`
			StructuredContent json.RawMessage `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		var echo, fail, noAccount result
		for _, c := range []struct {
			tool string
			args map[string]any
			into *result
		}{
			{"echo", map[string]any{"account_id": "acct-a"}, &echo},
			{"fail", map[string]any{"account_id": "acct-a"}, &fail},
			{"echo", map[string]any{}, &noAccount},
		} {
			r := call(t, h, e, "tools/call", c.tool, map[string]any{"name": c.tool, "arguments": c.args})
			if err := json.Unmarshal(r.Result, c.into); err != nil {
				t.Fatalf("%s at %s: %v", c.tool, e.name, err)
			}
		}
		if echo.IsError || string(echo.StructuredContent) != `{"account":"acct-a","input":{}}` {
			t.Errorf("echo at %s returned isError %v and %s", e.name, echo.IsError, echo.StructuredContent)
		}
		for name, r := range map[string]result{"fail": fail, "echo without an account": noAccount} {
			// The failure's structured content and its text are the service layer's, and say only
			// that the call failed (ADR-0086's R10).
			failure := `{"error":"the operation failed"}`
			if !r.IsError || string(r.StructuredContent) != failure || len(r.Content) != 1 || r.Content[0].Text != failure {
				t.Errorf("%s at %s returned isError %v, structured content %s and %+v", name, e.name, r.IsError, r.StructuredContent, r.Content)
			}
		}
		if r := call(t, h, e, "tools/call", "approve_plan", map[string]any{"name": "approve_plan", "arguments": map[string]any{}}); r.Error == nil {
			t.Errorf("a call at %s to a tool the registry does not hold answered %s", e.name, r.Result)
		}
	}
}

// An argument object holding a key twice, exactly or in another case, is refused before the operation
// runs, as the API root refuses it, so a handler never picks one of two values (ADR-0087).
func TestARepeatedArgumentIsRefused(t *testing.T) {
	var ran bool
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operation{
		Name: "get_thing", Description: "A fixture get_thing.", Effect: service.Read, Path: "/api/accounts/{account_id}/things",
		Input:  json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"},"label":{"type":"string"}},"required":["account_id"]}`),
		Output: json.RawMessage(`{"type":"object"}`),
		Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			ran = true
			return json.RawMessage(`{}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := mcp.Handler(reg, "test")
	for _, args := range []string{`{"account_id":"acct-a","label":"x","label":"y"}`, `{"account_id":"acct-a","label":"x","LABEL":"y"}`} {
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_thing","arguments":` + args + `}}`
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var got struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !got.Result.IsError {
			t.Errorf("arguments %s answered %s, want a tool error", args, rec.Body)
		}
		if ran {
			t.Errorf("arguments %s reached the operation", args)
		}
	}
}
