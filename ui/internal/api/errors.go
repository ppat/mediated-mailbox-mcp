package api

import (
	"encoding/json"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// The error contract's origins (docs/UI.md section 17.3), mirroring O5's distinction for the client
// surface. The client caused it, the UI server failed on its own, the database did not answer, or the
// provider did not answer a setup request.
const (
	originClient   = "client"
	originUI       = "ui"
	originDatabase = "database"
	originProvider = "provider"
)

// failure is one error the read API reports.
type failure struct {
	status  int
	origin  string
	code    string
	message string
}

func clientFault(status int, code, message string) *failure {
	return &failure{status: status, origin: originClient, code: code, message: message}
}

// uiFault is the UI server failing on its own. Its message names no detail, which goes to the log.
func uiFault() *failure {
	return &failure{status: http.StatusInternalServerError, origin: originUI, code: "internal", message: "the UI server failed"}
}

// databaseFault is the database not answering or refusing. A grant refusal names a table and nothing
// the operator can act on, so it maps here rather than to a client fault (ADR-0084).
func databaseFault() *failure {
	return &failure{status: http.StatusServiceUnavailable, origin: originDatabase, code: "database", message: "the database did not answer"}
}

// providerFault is the provider not answering a setup request, checking a client or exchanging a
// consent's code (docs/UI.md section 17.3). Nothing was stored.
func providerFault() *failure {
	return &failure{status: http.StatusBadGateway, origin: originProvider, code: "provider_unreachable", message: "the provider did not answer"}
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ErrorType is the error contract's declaration.
func ErrorType() schema.Type {
	return schema.Obj("Error", schema.F("error", schema.Obj("ErrorDetail",
		schema.F("origin", schema.Str(originClient, originUI, originDatabase, originProvider)),
		schema.F("code", schema.Str()),
		schema.F("message", schema.Str()),
		schema.F("request_id", schema.Str()),
	)))
}

// writeFailure writes f in the one shape every failure has, and records its origin for the request's
// log line.
func writeFailure(w http.ResponseWriter, r *http.Request, f *failure) {
	info := requestInfo(r.Context())
	info.origin, info.code = f.origin, f.code
	body, err := json.Marshal(errorBody{Error: errorDetail{Origin: f.origin, Code: f.code, Message: f.message, RequestID: info.id}})
	if err != nil {
		// The body is plain strings, so this is unreachable. The status still says what happened.
		info.err = err
		w.WriteHeader(f.status)
		return
	}
	send(w, r, f.status, body)
}

// writeJSON writes v as a 200 response, or a UI fault when v does not encode.
func writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, uiFault())
		return
	}
	send(w, r, http.StatusOK, body)
}

// writeBody writes v as a response of status, or a UI fault when v does not encode.
func writeBody(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, uiFault())
		return
	}
	send(w, r, status, body)
}

// send writes a JSON body. A failed write means the client went away, which the log line records.
func send(w http.ResponseWriter, r *http.Request, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := w.Write(append(body, '\n')); err != nil {
		requestInfo(r.Context()).err = err
	}
}
