package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// updating reports whether the run was given -update, which testsupport/compare defines for golden
// files. Here it writes the generated document to mediate/contract instead of comparing.
func updating() bool {
	f := flag.Lookup("update")
	return f != nil && f.Value.String() == "true"
}

// documentPath is the checked-in contract document, relative to this package's directory.
var documentPath = filepath.Join("..", "..", "contract", "openapi.json")

// document generates the API root's contract document from reg. Each operation is POST
// /api/<name>, with its input schema as the request body and its output schema as the response, and
// every operation requires the bearer token.
func document(t *testing.T, reg service.Registry) *openapi3.T {
	t.Helper()
	doc := &openapi3.T{
		OpenAPI: "3.1.1",
		Info:    &openapi3.Info{Title: "mediated-mailbox-mediate", Version: "0"},
		Paths:   openapi3.NewPaths(),
		Components: &openapi3.Components{SecuritySchemes: openapi3.SecuritySchemes{
			"bearer": &openapi3.SecuritySchemeRef{Value: openapi3.NewSecurityScheme().WithType("http").WithScheme("bearer")},
		}},
		Security: openapi3.SecurityRequirements{{"bearer": []string{}}},
	}
	for _, op := range reg.Operations() {
		input, output := schema(t, op.Input), schema(t, op.Output)
		operation := &openapi3.Operation{
			OperationID: op.Name,
			Description: op.Description,
			RequestBody: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().WithRequired(true).WithJSONSchema(input)},
			Responses:   openapi3.NewResponses(openapi3.WithStatus(http.StatusOK, &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("The operation's result.").WithJSONSchema(output)})),
		}
		doc.Paths.Set("/api/"+op.Name, &openapi3.PathItem{Post: operation})
	}
	return doc
}

// schema reads a registry schema into kin-openapi's type.
func schema(t *testing.T, raw json.RawMessage) *openapi3.Schema {
	t.Helper()
	s := openapi3.NewSchema()
	if err := json.Unmarshal(raw, s); err != nil {
		t.Fatalf("reading the schema %s: %v", raw, err)
	}
	return s
}

// render serializes the document in the form checked in, two-space indentation and one trailing
// newline, which the repository's text fixers leave unchanged.
func render(t *testing.T, doc *openapi3.T) []byte {
	t.Helper()
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("serializing the document: %v", err)
	}
	return append(out, '\n')
}

func TestDocument(t *testing.T) {
	reg, err := service.NewRegistry(nil, service.Operations()...)
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, reg)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("the generated document is not valid OpenAPI: %v", err)
	}
	got := render(t, doc)
	if updating() {
		if err := os.WriteFile(documentPath, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatalf("reading the checked-in document: %v. Generate it with: go generate ./mediate/internal/api", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from what the registry generates. Regenerate with: go generate ./mediate/internal/api\n--- generated\n%s", documentPath, got)
	}
}

// registry holds an operation that echoes its arguments and one that always fails with an error
// whose text must never reach the client.
func registry(t *testing.T) service.Registry {
	t.Helper()
	reg, err := service.NewRegistry([]string{"acct-a"},
		service.Operation{
			Name:        "echo",
			Description: "Returns its arguments.",
			Input:       json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"},"text":{"type":"string"}},"required":["account_id"]}`),
			Output:      json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			Handle: func(_ context.Context, account string, in json.RawMessage) (json.RawMessage, error) {
				return json.Marshal(map[string]any{"account": account, "input": in})
			},
		},
		service.Operation{
			Name:        "fail",
			Description: "Always fails.",
			Input:       json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
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

// answer is one response of the API root.
type answer struct {
	Status int
	Body   string
}

// send sends one request to h.
func send(t *testing.T, h http.Handler, method, path, body string) answer {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	got, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return answer{rec.Code, strings.TrimSpace(string(got))}
}

// The root serves each operation at POST /api/<name> and nothing else. A failed operation says only
// that it failed.
func TestTheRootServesTheRegistrysOperations(t *testing.T) {
	h := api.Handler(registry(t))
	cases := []struct {
		name, method, path, body string
		want                     answer
	}{
		{"an operation", http.MethodPost, "/api/echo", `{"account_id":"acct-a","text":"hello"}`, answer{200, `{"account":"acct-a","input":{"text":"hello"}}`}},
		{"a failing operation", http.MethodPost, "/api/fail", `{"account_id":"acct-a"}`, answer{500, `{"error":"the operation failed"}`}},
		{"an operation without its account", http.MethodPost, "/api/echo", `{"text":"hello"}`, answer{500, `{"error":"the operation failed"}`}},
		{"an operation not in the registry", http.MethodPost, "/api/approve_plan", `{}`, answer{404, `{"error":"no such operation"}`}},
		{"the root itself", http.MethodPost, "/api/", `{}`, answer{404, `{"error":"no such operation"}`}},
		{"a path below an operation", http.MethodPost, "/api/echo/more", `{}`, answer{404, `{"error":"no such operation"}`}},
		{"a path outside the root", http.MethodPost, "/echo", `{}`, answer{404, `{"error":"no such operation"}`}},
		{"another method", http.MethodGet, "/api/echo", ``, answer{405, `{"error":"an operation is called with POST"}`}},
		{"a body that is not JSON", http.MethodPost, "/api/echo", `{"text":`, answer{400, `{"error":"the request body is not a JSON object"}`}},
		{"a body that is not an object", http.MethodPost, "/api/echo", `["text"]`, answer{400, `{"error":"the request body is not a JSON object"}`}},
		{"a body that is null", http.MethodPost, "/api/echo", `null`, answer{400, `{"error":"the request body is not a JSON object"}`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, send(t, h, c.method, c.path, c.body), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// Both roots generated from one registry carry exactly its operations, the API root's as the contract
// document's paths and as what it serves, the MCP root's as its tool list (ADR-0030, ADR-0053).
func TestBothRootsCarryExactlyTheRegistry(t *testing.T) {
	reg := registry(t)
	var names []string
	for _, op := range reg.Operations() {
		names = append(names, op.Name)
	}

	var documented []string
	apiRoot := api.Handler(reg)
	for path := range document(t, reg).Paths.Map() {
		name, _ := strings.CutPrefix(path, "/api/")
		documented = append(documented, name)
		if got := send(t, apiRoot, http.MethodPost, path, `{}`); got.Status == http.StatusNotFound {
			t.Errorf("the API root does not serve %s, which the contract document carries", path)
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

	want := map[string][]string{"contract": names, "tools": names}
	got := map[string][]string{"contract": documented, "tools": tools}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("operations on each root (-want +got):\n%s", diff)
	}
}

// The contract document carries each operation's schemas as the registry holds them.
func TestTheDocumentCarriesTheRegistrysSchemas(t *testing.T) {
	reg := registry(t)
	doc := document(t, reg)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("the generated document is not valid OpenAPI: %v", err)
	}
	for _, op := range reg.Operations() {
		item := doc.Paths.Value("/api/" + op.Name)
		if item == nil || item.Post == nil {
			t.Fatalf("the document has no POST /api/%s", op.Name)
		}
		in, err := json.Marshal(item.Post.RequestBody.Value.Content.Get("application/json").Schema.Value)
		if err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(item.Post.Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema.Value)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"input": decode(t, op.Input), "output": decode(t, op.Output)}
		got := map[string]any{"input": decode(t, in), "output": decode(t, out)}
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("%s schemas (-want +got):\n%s", op.Name, diff)
		}
	}
}

func decode(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
