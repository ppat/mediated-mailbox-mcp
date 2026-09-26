package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// prefix is the path every operation is served under, followed by the operation's name.
const prefix = "/api/"

// maxRequestBytes bounds a request body.
const maxRequestBytes = 1 << 20

// Handler returns the API root generated from reg. It serves each operation at POST /api/<name>, with
// the operation's arguments as the JSON request body and its result as the JSON response body, and
// serves nothing else.
func Handler(reg service.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, found := strings.CutPrefix(r.URL.Path, prefix)
		op, known := reg.Lookup(name)
		if !found || !known {
			writeError(w, http.StatusNotFound, "no such operation")
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "an operation is called with POST")
			return
		}
		var input json.RawMessage
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(&input); err != nil || !isObject(input) {
			writeError(w, http.StatusBadRequest, "the request body is not a JSON object")
			return
		}
		out, err := reg.Call(r.Context(), op.Name, input)
		if err != nil {
			slog.ErrorContext(r.Context(), "operation failed", "operation", op.Name, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write(service.Failure(err)); err != nil {
				slog.WarnContext(r.Context(), "writing an error response failed", "error", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(out); err != nil {
			slog.WarnContext(r.Context(), "writing the response failed", "operation", op.Name, "error", err)
		}
	})
}

// isObject reports whether a decoded JSON value is an object.
func isObject(v json.RawMessage) bool {
	var m map[string]json.RawMessage
	return json.Unmarshal(v, &m) == nil && m != nil
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	body, err := json.Marshal(struct {
		Error string `json:"error"`
	}{message})
	if err != nil {
		body = []byte(`{"error":"internal"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Warn("writing an error response failed", "error", err)
	}
}
