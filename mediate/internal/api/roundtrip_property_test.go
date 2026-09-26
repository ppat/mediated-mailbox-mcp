package api_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// roundTrip is one generated call, the operation, and its whole argument object as JSON, account_id
// included where the operation takes one.
type roundTrip struct {
	Op   string
	Args string
}

// shapes are the declared types of each fixture operation's arguments, read from its schema, and
// which of them the path carries.
type shape struct {
	method   string
	path     string
	types    map[string]string
	required []string
	vars     []string
}

func shapes(t testing.TB, reg service.Registry) map[string]shape {
	t.Helper()
	out := map[string]shape{}
	for _, op := range reg.Operations() {
		if op.Name == "fail_messages" {
			continue
		}
		var in struct {
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(op.Input, &in); err != nil {
			t.Fatal(err)
		}
		s := shape{method: op.Method, path: op.Path, types: map[string]string{}, required: in.Required, vars: pathVars(op.Path)}
		for name, p := range in.Properties {
			s.types[name] = p.Type
		}
		out[op.Name] = s
	}
	return out
}

// drawRoundTrip draws an operation and an argument object its schema admits. A path variable is a
// non-empty string with no dot or slash, the account one the registry serves, and every other string
// is arbitrary text. An optional argument is present or absent at random.
func drawRoundTrip(shapes map[string]shape) func(*rapid.T) roundTrip {
	names := slices.Sorted(maps.Keys(shapes))
	return func(t *rapid.T) roundTrip {
		name := rapid.SampledFrom(names).Draw(t, "op")
		s := shapes[name]
		args := map[string]any{}
		for _, prop := range slices.Sorted(maps.Keys(s.types)) {
			if !slices.Contains(s.required, prop) && !rapid.Bool().Draw(t, "present "+prop) {
				continue
			}
			switch {
			case prop == "account_id":
				args[prop] = rapid.SampledFrom(served).Draw(t, prop)
			case slices.Contains(s.vars, prop):
				args[prop] = rapid.StringMatching(`[\p{L}\p{N} _~-]{1,12}`).Draw(t, prop)
			default:
				args[prop] = drawValue(t, prop, s.types[prop])
			}
		}
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		return roundTrip{Op: name, Args: string(raw)}
	}
}

// drawValue draws a value of a declared JSON type.
func drawValue(t *rapid.T, label, typ string) any {
	switch typ {
	case "string":
		return rapid.String().Draw(t, label)
	case "integer":
		return rapid.Int64Range(-1<<53, 1<<53).Draw(t, label)
	case "number":
		return rapid.Float64Range(-1e12, 1e12).Draw(t, label)
	case "boolean":
		return rapid.Bool().Draw(t, label)
	case "array":
		return rapid.SliceOfN(rapid.String(), 0, 4).Draw(t, label)
	default:
		return rapid.MapOfN(rapid.StringMatching(`[a-z_]{1,6}`), rapid.OneOf(
			rapid.Map(rapid.String(), func(s string) any { return s }),
			rapid.Map(rapid.Bool(), func(b bool) any { return b }),
			rapid.Map(rapid.SliceOfN(rapid.String(), 0, 3), func(s []string) any { return s }),
		), 0, 4).Draw(t, label)
	}
}

// apiRequest is the API root's request for a generated call. The path variables are escaped into
// the path, a GET's other arguments go to the query string as text, and a POST's to the body.
func apiRequest(t rapid.TB, s shape, args map[string]any) *http.Request {
	path := s.path
	query := url.Values{}
	body := map[string]any{}
	for name, v := range args {
		switch {
		case slices.Contains(s.vars, name):
			str, ok := v.(string)
			if !ok {
				t.Fatalf("the path variable %s is %v, not a string", name, v)
			}
			path = strings.Replace(path, "{"+name+"}", url.PathEscape(str), 1)
		case s.method == http.MethodGet:
			query.Set(name, queryText(s.types[name], v))
		default:
			body[name] = v
		}
	}
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var payload []byte
	if s.method == http.MethodPost {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}
	return httptest.NewRequest(s.method, target, bytes.NewReader(payload))
}

// queryText writes a scalar of a declared type as a query parameter's text, as a client does. The
// argument object has passed through JSON, so every number arrives as a float64.
func queryText(typ string, v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if typ == "integer" {
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

// The object reaching Registry.Call is the same from both roots, for every operation, although the
// API root carries the account and a read's arguments in the path and the query string (ADR-0087,
// ADR-0030). The operation receives the drawn object's account and the rest of its arguments.
func TestBothRootsHandTheServiceTheSameObject(t *testing.T) {
	reg, rec := fixtures(t)
	shapes := shapes(t, reg)
	apiRoot := root(t, reg)
	mcpRoot := mcp.Handler(reg, "test")
	property.Check(t, drawRoundTrip(shapes), func(t rapid.TB, c roundTrip) {
		var args map[string]any
		if err := json.Unmarshal([]byte(c.Args), &args); err != nil {
			t.Fatal(err)
		}
		rec.take()

		w := httptest.NewRecorder()
		apiRoot.ServeHTTP(w, apiRequest(t, shapes[c.Op], args))
		if w.Code != http.StatusOK {
			t.Fatalf("%+v: the API root answered %d %s", c, w.Code, w.Body)
		}
		viaAPI := rec.take()

		req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": c.Op, "arguments": json.RawMessage(c.Args)}})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(req))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		w = httptest.NewRecorder()
		mcpRoot.ServeHTTP(w, r)
		viaMCP := rec.take()

		account := ""
		if a, ok := args["account_id"].(string); ok {
			account = a
		}
		rest := map[string]any{}
		for k, v := range args {
			if k != "account_id" || c.Op == "list_accounts" {
				rest[k] = v
			}
		}
		var want any
		raw, err := json.Marshal(rest)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		expected := []call{{c.Op, account, want}}
		if diff := cmp.Diff(map[string][]call{"api": expected, "mcp": expected}, map[string][]call{"api": viaAPI, "mcp": viaMCP}, compare.Options); diff != "" {
			t.Fatalf("%+v: the calls reaching the operation (-want +got):\n%s", c, diff)
		}
	})
}

// The generated calls cover every fixture operation, and so every effect class, in a useful share.
func TestBothRootsHandTheServiceTheSameObjectMix(t *testing.T) {
	reg, _ := fixtures(t)
	shapes := shapes(t, reg)
	minimums := map[string]float64{}
	for name := range shapes {
		minimums[name] = 0.05
	}
	property.Report(t, drawRoundTrip(shapes), func(c roundTrip) string { return c.Op }, minimums)
}
