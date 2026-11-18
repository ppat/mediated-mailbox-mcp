package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/bearer"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// Build builds the API server's API endpoint, the API root generated from the process's one registry
// of operations behind the bearer check and the no-store header (ADR-0030, ADR-0087, ADR-0142).
func Build(ctx context.Context, p process.Capabilities, c config.API) (http.Handler, error) {
	reg, err := registry.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	root, err := Handler(reg, p.Logger())
	if err != nil {
		return nil, err
	}
	return bearer.Wrap(ctx, p, c, root)
}

// maxRequestBytes bounds a request body.
const maxRequestBytes = 1 << 20

// Handler returns the API root generated from reg (ADR-0087). Each operation is served at its
// derived method and its path template, and nothing else is served. A HEAD request is refused on
// every route. The root rebuilds the operation's one argument object from the path, the query string
// typed by the input schema, and the body, and hands it to Registry.Call.
//
// It returns an error when two operations' routes conflict, which the registry's own checks leave to
// the router. It logs each failed call through logger, at the level its origin sets.
func Handler(reg service.Registry, logger *slog.Logger) (h http.Handler, err error) {
	mux := http.NewServeMux()
	defer func() {
		if p := recover(); p != nil {
			h, err = nil, fmt.Errorf("the API root's routes conflict: %v", p)
		}
	}()
	for _, op := range reg.Operations() {
		route := route{op: op, logger: logger}
		mux.Handle(op.Method+" "+op.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route.serve(reg, w, r)
		}))
	}
	return mux, nil
}

// route is one operation's binding from a request to its argument object, which reads the path
// variables and the declared argument types the registry read from the operation.
type route struct {
	op     service.Descriptor
	logger *slog.Logger
}

// member is one member of the argument object, its key and its JSON value.
type member struct {
	key   string
	value json.RawMessage
}

// errBadRequest marks a request the root cannot turn into an argument object.
var errBadRequest = errors.New("bad request")

func (rt route) serve(reg service.Registry, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.Header().Set("Allow", rt.op.Method)
		writeError(w, rt.logger, http.StatusMethodNotAllowed, "HEAD is not served")
		return
	}
	input, err := rt.arguments(w, r)
	if err != nil {
		writeError(w, rt.logger, http.StatusBadRequest, err.Error())
		return
	}
	out, err := reg.Call(r.Context(), rt.op.Name, input)
	if err != nil {
		failure := service.Failed(r.Context(), rt.logger, rt.op.Name, err)
		origin, _ := service.Classify(err)
		writeFailure(w, rt.logger, status(origin), failure)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(out); err != nil { //nolint:gosec // a JSON body sent as application/json, never rendered as HTML
		rt.logger.DebugContext(r.Context(), "writing the response failed", "operation", rt.op.Name, "error", err)
	}
}

// arguments rebuilds the operation's argument object from the path, the query string and the body.
// Each argument has one location. A path variable comes from the path, and every other argument of
// a GET from the query string, typed by its declared type, and of a POST from the body. An account_id
// given in the query string or the body beside the path's is kept, so the service layer refuses the
// ambiguous account. Any other argument given twice is refused here.
func (rt route) arguments(w http.ResponseWriter, r *http.Request) (json.RawMessage, error) {
	var members []member
	for _, name := range rt.op.PathVariables {
		value, err := json.Marshal(r.PathValue(name))
		if err != nil {
			return nil, fmt.Errorf("%w: path variable %s", errBadRequest, name)
		}
		members = append(members, member{name, value})
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, errors.New("the query string cannot be read")
	}
	for _, key := range slices.Sorted(maps.Keys(query)) {
		values := query[key]
		if len(values) != 1 {
			return nil, fmt.Errorf("the query parameter %q is given more than once", key)
		}
		value, err := rt.queryValue(key, values[0])
		if err != nil {
			return nil, err
		}
		members = append(members, member{key, value})
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		return nil, errors.New("the request body cannot be read")
	}
	switch rt.op.Method {
	case http.MethodGet:
		if len(bytes.TrimSpace(body)) > 0 {
			return nil, errors.New("a GET takes no request body")
		}
	default:
		bodyMembers, err := objectMembers(body)
		if err != nil {
			return nil, errors.New("the request body is not a JSON object")
		}
		members = append(members, bodyMembers...)
	}
	seen := map[string]bool{}
	var object bytes.Buffer
	object.WriteByte('{')
	for i, m := range members {
		if !strings.EqualFold(m.key, "account_id") {
			if seen[m.key] {
				return nil, fmt.Errorf("the argument %q is given more than once", m.key)
			}
			seen[m.key] = true
		}
		if i > 0 {
			object.WriteByte(',')
		}
		key, err := json.Marshal(m.key)
		if err != nil {
			return nil, errBadRequest
		}
		object.Write(key)
		object.WriteByte(':')
		object.Write(m.value)
	}
	object.WriteByte('}')
	return object.Bytes(), nil
}

// queryValue decodes one query parameter as its declared type. A key the input does not declare is
// refused, except an account_id on a route whose path names the account, which is kept as a string
// for the service layer to refuse as ambiguous. A POST takes no query parameter but that one.
func (rt route) queryValue(key, text string) (json.RawMessage, error) {
	if strings.EqualFold(key, "account_id") && rt.isPathVar("account_id") {
		return json.Marshal(text)
	}
	t, declared := rt.op.ArgumentTypes[key]
	if !declared || rt.op.Method != http.MethodGet || rt.isPathVar(key) {
		return nil, fmt.Errorf("the query parameter %q is not an argument the query string carries", key)
	}
	switch t {
	case "string":
		return json.Marshal(text)
	case "integer":
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the query parameter %q is not an integer", key)
		}
		return json.Marshal(n)
	case "number":
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("the query parameter %q is not a number", key)
		}
		return json.Marshal(f)
	case "boolean":
		switch text {
		case "true", "false":
			return json.RawMessage(text), nil
		}
		return nil, fmt.Errorf("the query parameter %q is not true or false", key)
	default:
		return nil, fmt.Errorf("the query parameter %q has no scalar type", key)
	}
}

func (rt route) isPathVar(key string) bool { return slices.Contains(rt.op.PathVariables, key) }

// objectMembers returns a JSON object's members in order, keys as written, or an error when body is
// not one JSON object.
func objectMembers(body []byte) ([]member, error) {
	obj, ok := service.OpenObject(body)
	if !ok {
		return nil, errBadRequest
	}
	var members []member
	for obj.More() {
		key, value, ok := obj.Next()
		if !ok {
			return nil, errBadRequest
		}
		members = append(members, member{key, value})
	}
	if !obj.End() {
		return nil, errBadRequest
	}
	return members, nil
}

// status returns the HTTP status a failure of origin answers with (ADR-0101).
func status(origin service.Origin) int {
	switch origin {
	case service.OriginClient:
		return http.StatusBadRequest
	case service.OriginProvider:
		return http.StatusBadGateway
	case service.OriginAPI:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// writeError writes the root's own refusal of a request it cannot turn into a call, a failure of the
// client's, in the shape every failure takes (ADR-0101).
func writeError(w http.ResponseWriter, logger *slog.Logger, status int, message string) {
	writeFailure(w, logger, status, service.Failure(service.Refuse(message)))
}

// writeFailure writes a failure's content with status. A failed write means the client went away,
// which leaves the operator nothing to act on, so it is detail.
func writeFailure(w http.ResponseWriter, logger *slog.Logger, status int, body json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil { //nolint:gosec // a JSON body sent as application/json, never rendered as HTML
		logger.Debug("writing an error response failed", "error", err)
	}
}
