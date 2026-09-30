package contract_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// documentPath is the checked-in document, relative to this package's directory.
var documentPath = filepath.Join("..", "..", "contract", "openapi.json")

// generate builds the document from the registry and the handler list, the two sources ADR-0057
// names, by populating kin-openapi's document types explicitly (ADR-0065). It refuses a path both
// sources claim.
func generate(datasets []registry.Dataset, bespoke []api.Route) (*openapi3.T, error) {
	g := &generator{components: map[string]schema.Type{}, doc: &openapi3.T{
		OpenAPI:    "3.1.1",
		Info:       &openapi3.Info{Title: "mediated-mailbox-ui", Version: "0"},
		Paths:      openapi3.NewPaths(),
		Components: &openapi3.Components{Schemas: openapi3.Schemas{}},
	}}
	g.schema(api.ErrorType())
	g.doc.Paths.Set(registry.LensPath, &openapi3.PathItem{Get: g.lensOperation(datasets)})
	if err := api.CheckPaths(bespoke); err != nil {
		return nil, err
	}
	for _, route := range bespoke {
		g.doc.Paths.Set(route.Pattern, &openapi3.PathItem{Get: g.bespokeOperation(route)})
	}
	if g.err != nil {
		return nil, g.err
	}
	return g.doc, nil
}

type generator struct {
	doc        *openapi3.T
	components map[string]schema.Type
	err        error
}

// errorResponses are the error contract's statuses (docs/UI.md section 17.3).
func (g *generator) errorResponses(op *openapi3.Operation) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		op.AddResponse(status, openapi3.NewResponse().
			WithDescription(http.StatusText(status)).
			WithJSONSchemaRef(openapi3.NewSchemaRef("#/components/schemas/Error", g.doc.Components.Schemas["Error"].Value)))
	}
}

func accountParameter() *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: openapi3.NewPathParameter("account").WithSchema(openapi3.NewStringSchema()).
		WithDescription("The account every read is scoped to. all and an unknown account are refused")}
}

func (g *generator) bespokeOperation(route api.Route) *openapi3.Operation {
	op := openapi3.NewOperation()
	op.OperationID = route.Operation
	op.Summary = route.Summary
	if route.Scoped {
		op.Parameters = append(op.Parameters, accountParameter())
	}
	ok := openapi3.NewResponse().WithDescription("OK")
	if route.Stream() {
		ok.Content = openapi3.NewContentWithSchema(openapi3.NewStringSchema(), []string{"text/event-stream"})
		events := map[string]string{}
		names := make([]string, 0, len(route.Events))
		for name := range route.Events {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			t := route.Events[name]
			g.schema(t)
			events[name] = "#/components/schemas/" + t.Name
		}
		op.Extensions = map[string]any{"x-events": events}
	} else {
		ok.Content = openapi3.NewContentWithJSONSchemaRef(g.schema(route.Response))
	}
	op.AddResponse(http.StatusOK, ok)
	g.errorResponses(op)
	return op
}

// lensOperation is the dataset endpoint. Its query parameters are the common ones and every filter
// the registry declares, and its x-datasets extension carries each entry's declarative half, from
// which the browser's descriptor table is generated (ADR-0065).
func (g *generator) lensOperation(datasets []registry.Dataset) *openapi3.Operation {
	op := openapi3.NewOperation()
	op.OperationID = "getLens"
	op.Summary = "One dataset at one level of the zoom ladder, for one account"
	names := make([]any, len(datasets))
	for i, d := range datasets {
		names[i] = d.Name
	}
	query := func(name, description string, s *openapi3.Schema) *openapi3.ParameterRef {
		return &openapi3.ParameterRef{Value: openapi3.NewQueryParameter(name).WithSchema(s).WithDescription(description)}
	}
	dataset := openapi3.NewStringSchema()
	dataset.Enum = names
	op.Parameters = openapi3.Parameters{
		accountParameter(),
		{Value: openapi3.NewQueryParameter("dataset").WithSchema(dataset).WithDescription("The registry's dataset").WithRequired(true)},
		query("level", "0 to 3", openapi3.NewIntegerSchema().WithMin(lens.Summary).WithMax(lens.Rows)),
		query("group", "One groupable dimension, at levels 1 and 2", openapi3.NewStringSchema()),
		query("range", "A preset or two UTC dates from,to", openapi3.NewStringSchema()),
		query("sort", "column,asc or column,desc", openapi3.NewStringSchema()),
		query("page", "The 1-based page of 50 rows", openapi3.NewIntegerSchema().WithMin(1)),
	}
	var filters []string
	for _, d := range datasets {
		for _, dim := range d.Dimensions {
			if dim.Filterable && !slices.Contains(filters, dim.Name) {
				filters = append(filters, dim.Name)
			}
		}
		if d.Parent != "" && !slices.Contains(filters, d.Parent) {
			filters = append(filters, d.Parent)
		}
	}
	slices.Sort(filters)
	for _, f := range filters {
		op.Parameters = append(op.Parameters, query(f, "A dimension filter, value, a,b or !value", openapi3.NewStringSchema()))
	}
	op.AddResponse(http.StatusOK, openapi3.NewResponse().WithDescription("OK").
		WithContent(openapi3.NewContentWithJSONSchemaRef(g.schema(api.LensResponse(datasets)))))
	g.errorResponses(op)
	op.Extensions = map[string]any{"x-datasets": descriptors(datasets)}
	return op
}

// descriptor is one registry entry's declarative half as the document carries it.
type descriptor struct {
	Name        string      `json:"name"`
	Parent      string      `json:"parent,omitempty"`
	Ranged      bool        `json:"ranged"`
	RangeColumn string      `json:"range_column,omitempty"`
	Dimensions  []dimension `json:"dimensions"`
	Default     defaults    `json:"default"`
}

type dimension struct {
	Name        string   `json:"name"`
	Storage     string   `json:"storage"`
	Groupable   bool     `json:"groupable"`
	Filterable  bool     `json:"filterable"`
	Sortable    bool     `json:"sortable"`
	Wording     string   `json:"wording"`
	NullWording string   `json:"null_wording,omitempty"`
	Values      []string `json:"values,omitempty"`
}

type defaults struct {
	Group   string              `json:"group,omitempty"`
	Level   int                 `json:"level"`
	Range   string              `json:"range,omitempty"`
	Sort    string              `json:"sort"`
	Filters map[string][]string `json:"filters,omitempty"`
}

func descriptors(datasets []registry.Dataset) []descriptor {
	out := make([]descriptor, 0, len(datasets))
	for _, d := range datasets {
		ds := descriptor{
			Name: d.Name, Parent: d.Parent, Ranged: d.Ranged, RangeColumn: d.RangeColumn,
			Default: defaults{Group: d.Default.Group, Level: d.Default.Level, Range: d.Default.Range, Sort: d.Default.Sort.String(), Filters: d.Default.Filters},
		}
		for _, dim := range d.Dimensions {
			ds.Dimensions = append(ds.Dimensions, dimension{
				Name: dim.Name, Storage: dim.Storage, Groupable: dim.Groupable, Filterable: dim.Filterable, Sortable: dim.Sortable,
				Wording: dim.Wording, NullWording: dim.NullWording, Values: dim.Values,
			})
		}
		out = append(out, ds)
	}
	return out
}

// schema converts a declaration, registering each named object as a component and referencing it.
// Every object declares all its fields as required and admits no other.
func (g *generator) schema(t schema.Type) *openapi3.SchemaRef {
	if t.Name != "" {
		if prior, ok := g.components[t.Name]; ok {
			if !reflect.DeepEqual(prior, withNullable(t, prior.Nullable)) {
				g.err = fmt.Errorf("the component %s is declared twice with different shapes", t.Name)
			}
		} else {
			g.components[t.Name] = withNullable(t, false)
			named := t
			named.Name, named.Nullable = "", false
			g.doc.Components.Schemas[t.Name] = g.schema(named)
		}
		ref := openapi3.NewSchemaRef("#/components/schemas/"+t.Name, g.doc.Components.Schemas[t.Name].Value)
		if t.Nullable {
			s := openapi3.NewSchema()
			s.AnyOf = openapi3.SchemaRefs{ref, openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeNull}})}
			return openapi3.NewSchemaRef("", s)
		}
		return ref
	}
	s := openapi3.NewSchema()
	kind := ""
	switch t.Kind {
	case schema.String:
		kind = openapi3.TypeString
		for _, v := range t.Values {
			s.Enum = append(s.Enum, v)
		}
	case schema.Timestamp:
		kind = openapi3.TypeString
		s.Format = "date-time"
	case schema.Integer:
		kind = openapi3.TypeInteger
	case schema.Number:
		kind = openapi3.TypeNumber
	case schema.Boolean:
		kind = openapi3.TypeBoolean
	case schema.Array:
		kind = openapi3.TypeArray
		s.Items = g.schema(*t.Items)
	case schema.Map:
		kind = openapi3.TypeObject
		s.AdditionalProperties = openapi3.AdditionalProperties{Schema: g.schema(*t.Items)}
	case schema.Object:
		if len(t.Variants) > 0 {
			for _, v := range t.Variants {
				s.OneOf = append(s.OneOf, g.schema(v))
			}
			return openapi3.NewSchemaRef("", s)
		}
		kind = openapi3.TypeObject
		s.Properties = openapi3.Schemas{}
		no := false
		s.AdditionalProperties = openapi3.AdditionalProperties{Has: &no}
		for _, f := range t.Fields {
			s.Properties[f.Name] = g.schema(f.Type)
			s.Required = append(s.Required, f.Name)
		}
	case schema.JSON:
		// Any JSON value, null included. kin-openapi's validator reads a schema with no type as
		// refusing null, so every type is named.
		s.Type = &openapi3.Types{openapi3.TypeObject, openapi3.TypeArray, openapi3.TypeString, openapi3.TypeNumber, openapi3.TypeBoolean, openapi3.TypeNull}
		return openapi3.NewSchemaRef("", s)
	default:
		g.err = fmt.Errorf("a declaration of unknown kind %d", t.Kind)
		return openapi3.NewSchemaRef("", s)
	}
	types := openapi3.Types{kind}
	if t.Nullable {
		types = append(types, openapi3.TypeNull)
		if len(s.Enum) > 0 {
			s.Enum = append(s.Enum, nil)
		}
	}
	s.Type = &types
	return openapi3.NewSchemaRef("", s)
}

func withNullable(t schema.Type, nullable bool) schema.Type {
	t.Nullable = nullable
	return t
}

// render serializes the document in the form checked in, two-space indentation and one trailing newline,
// which the repository's text fixers leave unchanged.
func render(t *testing.T, doc *openapi3.T) []byte {
	t.Helper()
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("serializing the document: %v", err)
	}
	return append(out, '\n')
}

// TestDocument generates the document from the registry and the handler list, requires it valid, and
// compares it with the checked-in file, the first of the pipeline's three drift checks (ADR-0065). Run
// with -update, it writes the file.
func TestDocument(t *testing.T) {
	doc, err := generate(registry.Datasets(), api.Bespoke())
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("the generated document is not valid OpenAPI: %v", err)
	}
	compare.GoldenAt(t, documentPath, render(t, doc))
}

// TestAPathClaimedByBothSourcesIsRefused hands the generator a bespoke handler claiming the registry's
// path, and requires the refusal, so no route can be served by two definitions (docs/UI.md section
// 17.1).
func TestAPathClaimedByBothSourcesIsRefused(t *testing.T) {
	claimed := append(api.Bespoke(), api.Route{Pattern: registry.LensPath, Operation: "claimsTheLens", Scoped: true, Response: schema.Obj("Claim")})
	_, err := generate(registry.Datasets(), claimed)
	if err == nil || !strings.Contains(err.Error(), "claimed by both the registry and the bespoke handler claimsTheLens") {
		t.Fatalf("a path both sources claim was not refused: %v", err)
	}
	twice := append(api.Bespoke(), api.Bespoke()[0])
	if _, err := generate(registry.Datasets(), twice); err == nil || !strings.Contains(err.Error(), "claimed by two bespoke handlers") {
		t.Fatalf("a path two bespoke handlers claim was not refused: %v", err)
	}
}

// catchAlls are the two patterns the server registers beside the document's operations. /api/ refuses
// every path under /api that no operation answers, and / serves the entry document and the bundle.
// Each is named whole, so no other pattern passes by sharing a prefix with one.
var catchAlls = []string{"/api/", "/"}

// TestTheServerServesExactlyTheDocumentsRoutes builds the server and requires every pattern it
// registered through the UI listener's recording mux to be an operation of the checked-in document or
// one of the two catch-alls, and each of those to be registered. The document is read from the file
// rather than from the lists the server is built from, so a route registered through the recording mux
// that the document lacks is caught, under /api or outside it (ADR-0057).
func TestTheServerServesExactlyTheDocumentsRoutes(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile(documentPath)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(catchAlls)
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			want = append(want, method+" "+path)
		}
	}
	if got := sorted(newServer(t).Served()); !slices.Equal(got, sorted(want)) {
		t.Fatalf("the server registers %v, and the document's operations with the catch-alls %v are %v", got, catchAlls, sorted(want))
	}
}

// probeRoutes are the probes listener's three routes (ADR-0051), the only ones it may serve.
var probeRoutes = []string{"GET /healthz", "GET /metrics", "GET /readyz"}

// TestTheProbesListenerServesOnlyTheProbes requires every pattern the server registered through the
// probes listener's recording mux to be one of the three probe routes, and each of those to be
// registered, so a route added to the probes listener is caught as a route added to the UI's is.
func TestTheProbesListenerServesOnlyTheProbes(t *testing.T) {
	if got := sorted(newServer(t).ProbesServed()); !slices.Equal(got, probeRoutes) {
		t.Fatalf("the probes listener registers %v, and its routes are %v", got, probeRoutes)
	}
}

// newServer builds the server. It touches no database while it is built, so it is given none.
func newServer(t *testing.T) *api.Server {
	t.Helper()
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{}, Datasets: registry.Datasets(), Logger: slog.New(slog.DiscardHandler),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
