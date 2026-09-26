package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"

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

// inputSchema is what the document generator reads of an input schema.
type inputSchema struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

// document generates the API root's contract document from reg (ADR-0087). Each operation sits at its
// path under its derived method, its operationId the operation's name. A path variable is a path
// parameter. Every other argument of a GET is a query parameter, and of a POST a member of the
// request body, whose schema is the input schema without the path variables. The response is the
// output schema, and every operation requires the bearer token.
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
		var in inputSchema
		if err := json.Unmarshal(op.Input, &in); err != nil {
			t.Fatal(err)
		}
		vars := pathVars(op.Path)
		operation := &openapi3.Operation{
			OperationID: op.Name,
			Description: op.Description,
			Responses:   openapi3.NewResponses(openapi3.WithStatus(http.StatusOK, &openapi3.ResponseRef{Value: openapi3.NewResponse().WithDescription("The operation's result.").WithJSONSchema(schema(t, op.Output))})),
		}
		for _, v := range vars {
			operation.AddParameter(openapi3.NewPathParameter(v).WithSchema(schema(t, in.Properties[v])))
		}
		body := inputSchema{Properties: map[string]json.RawMessage{}}
		for _, name := range slices.Sorted(maps.Keys(in.Properties)) {
			if slices.Contains(vars, name) {
				continue
			}
			required := slices.Contains(in.Required, name)
			if op.Method == http.MethodGet {
				operation.AddParameter(openapi3.NewQueryParameter(name).WithRequired(required).WithSchema(schema(t, in.Properties[name])))
				continue
			}
			body.Properties[name] = in.Properties[name]
			if required {
				body.Required = append(body.Required, name)
			}
		}
		if op.Method == http.MethodPost {
			raw, err := json.Marshal(map[string]any{"type": "object", "properties": body.Properties, "required": body.Required})
			if err != nil {
				t.Fatal(err)
			}
			operation.RequestBody = &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().WithRequired(true).WithJSONSchema(schema(t, raw))}
		}
		doc.AddOperation(op.Path, op.Method, operation)
	}
	return doc
}

// pathVars returns the variables a path template names, in order.
func pathVars(path string) []string {
	var vars []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			vars = append(vars, strings.Trim(seg, "{}"))
		}
	}
	return vars
}

// schema reads a schema into kin-openapi's type.
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

// parameter is what a test observes of one parameter in the document.
type parameter struct {
	In, Name string
	Required bool
}

// The contract document carries each operation at its derived method and path, its path variables
// as path parameters, a GET's other arguments as query parameters, and a POST's as the request body
// without the path variables (ADR-0087).
func TestTheDocumentPlacesEachArgument(t *testing.T) {
	reg, _ := fixtures(t)
	doc := document(t, reg)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("the generated document is not valid OpenAPI: %v", err)
	}
	type placed struct {
		Method, Path string
		Parameters   []parameter
		Body         any
	}
	got := map[string]placed{}
	for path, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			p := placed{Method: method, Path: path}
			for _, ref := range o.Parameters {
				p.Parameters = append(p.Parameters, parameter{ref.Value.In, ref.Value.Name, ref.Value.Required})
			}
			if o.RequestBody != nil {
				raw, err := json.Marshal(o.RequestBody.Value.Content.Get("application/json").Schema.Value)
				if err != nil {
					t.Fatal(err)
				}
				p.Body = decode(t, raw)
			}
			got[o.OperationID] = p
		}
	}
	accountParam := parameter{"path", "account_id", true}
	want := map[string]placed{
		"list_accounts": {Method: "GET", Path: "/api/accounts", Parameters: []parameter{{"query", "page_size", false}}},
		"get_message": {Method: "GET", Path: "/api/accounts/{account_id}/messages/{message_id}", Parameters: []parameter{
			accountParam, {"path", "message_id", true}, {"query", "include_snippet", false}, {"query", "label", false}, {"query", "limit", false}, {"query", "score", false},
		}},
		"sample_reorg_plan": {Method: "GET", Path: "/api/accounts/{account_id}/reorg-plans/{plan_id}/sample", Parameters: []parameter{
			accountParam, {"path", "plan_id", true}, {"query", "size", false},
		}},
		"search_messages": {
			Method: "POST", Path: "/api/accounts/{account_id}/messages:search", Parameters: []parameter{accountParam},
			Body: decode(t, []byte(`{"type":"object","properties":{"cursor":{"type":"string"},"query":{"type":"object"}},"required":["query"]}`)),
		},
		"create_label": {
			Method: "POST", Path: "/api/accounts/{account_id}/labels", Parameters: []parameter{accountParam},
			Body: decode(t, []byte(`{"type":"object","properties":{"dry_run":{"type":"boolean"},"name":{"type":"string"}},"required":["name"]}`)),
		},
		"label_messages": {
			Method: "POST", Path: "/api/accounts/{account_id}/messages:label", Parameters: []parameter{accountParam},
			Body: decode(t, []byte(`{"type":"object","properties":{"dry_run":{"type":"boolean"},"label":{"type":"string"},"message_ids":{"type":"array","items":{"type":"string"}}},"required":["label","message_ids"]}`)),
		},
		"label_plan_items": {
			Method: "POST", Path: "/api/accounts/{account_id}/reorg-plans/{plan_id}/items:label", Parameters: []parameter{accountParam, {"path", "plan_id", true}},
			Body: decode(t, []byte(`{"type":"object","properties":{"label":{"type":"string"}},"required":["label"]}`)),
		},
		"trash_messages": {
			Method: "POST", Path: "/api/accounts/{account_id}/messages:trash", Parameters: []parameter{accountParam},
			Body: decode(t, []byte(`{"type":"object","properties":{"dry_run":{"type":"boolean"},"message_ids":{"type":"array","items":{"type":"string"}}},"required":["message_ids"]}`)),
		},
		"fail_messages": {
			Method: "POST", Path: "/api/accounts/{account_id}/messages:fail", Parameters: []parameter{accountParam},
			Body: decode(t, []byte(`{"type":"object"}`)),
		},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("placement (-want +got):\n%s", diff)
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
