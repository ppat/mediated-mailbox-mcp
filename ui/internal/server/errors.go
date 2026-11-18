package server

import (
	"encoding/json"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
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

// Failure is one error the UI reports, in the error contract's shape.
type Failure struct {
	status  int
	origin  string
	code    string
	message string
}

// statuses are the status each refusal's code is answered with, one per code wherever it is refused.
var statuses = map[refusal.Code]int{
	refusal.AllAccounts:          http.StatusBadRequest,
	refusal.APIDisabled:          http.StatusBadRequest,
	refusal.AttemptExpired:       http.StatusBadRequest,
	refusal.ClientRefused:        http.StatusBadRequest,
	refusal.ClientWrongProvider:  http.StatusBadRequest,
	refusal.CodeRefused:          http.StatusBadRequest,
	refusal.ConfirmationRequired: http.StatusBadRequest,
	refusal.ConsentDeclined:      http.StatusBadRequest,
	refusal.FileRefused:          http.StatusBadRequest,
	refusal.IdentifierRefused:    http.StatusBadRequest,
	refusal.InvalidFilter:        http.StatusBadRequest,
	refusal.InvalidLevel:         http.StatusBadRequest,
	refusal.InvalidPage:          http.StatusBadRequest,
	refusal.InvalidRange:         http.StatusBadRequest,
	refusal.InvalidRow:           http.StatusBadRequest,
	refusal.InvalidSort:          http.StatusBadRequest,
	refusal.MailboxRefused:       http.StatusBadRequest,
	refusal.MailboxRemembered:    http.StatusBadRequest,
	refusal.MailboxRequired:      http.StatusBadRequest,
	refusal.MalformedBody:        http.StatusBadRequest,
	refusal.MissingDataset:       http.StatusBadRequest,
	refusal.MissingFilter:        http.StatusBadRequest,
	refusal.MissingGroup:         http.StatusBadRequest,
	refusal.MissingParent:        http.StatusBadRequest,
	refusal.NameRefused:          http.StatusBadRequest,
	refusal.NoAttempt:            http.StatusBadRequest,
	refusal.NoCode:               http.StatusBadRequest,
	refusal.ProjectRefused:       http.StatusBadRequest,
	refusal.RepeatedParameter:    http.StatusBadRequest,
	refusal.RuleRefused:          http.StatusBadRequest,
	refusal.ScopeMissing:         http.StatusBadRequest,
	refusal.ScopeRefused:         http.StatusBadRequest,
	refusal.TargetRefused:        http.StatusBadRequest,
	refusal.UnknownAccount:       http.StatusBadRequest,
	refusal.UnknownDataset:       http.StatusBadRequest,
	refusal.UnknownDimension:     http.StatusBadRequest,
	refusal.UnknownGroup:         http.StatusBadRequest,
	refusal.UnknownParameter:     http.StatusBadRequest,
	refusal.UnknownSort:          http.StatusBadRequest,
	refusal.WrongAddress:         http.StatusBadRequest,
	refusal.WrongAttempt:         http.StatusBadRequest,
	refusal.WrongMailbox:         http.StatusBadRequest,
	refusal.IdentityMissing:      http.StatusForbidden,
	refusal.StalePage:            http.StatusForbidden,
	refusal.NotFound:             http.StatusNotFound,
	refusal.UnknownClient:        http.StatusNotFound,
	refusal.UnknownProvider:      http.StatusNotFound,
	refusal.UnknownRow:           http.StatusNotFound,
	refusal.UnknownRule:          http.StatusNotFound,
	refusal.UnknownRun:           http.StatusNotFound,
	refusal.ClientChanged:        http.StatusConflict,
	refusal.ClientExists:         http.StatusConflict,
	refusal.ClientInUse:          http.StatusConflict,
	refusal.IdentifierTaken:      http.StatusConflict,
	refusal.NameTaken:            http.StatusConflict,
	refusal.NotConnected:         http.StatusConflict,
	refusal.StalePreview:         http.StatusConflict,
	refusal.StaleRule:            http.StatusConflict,
}

// Status returns the status a refusal of code is answered with, and a bad request for a code the table
// does not hold, which its test refuses.
func Status(code refusal.Code) int {
	if s, ok := statuses[code]; ok {
		return s
	}
	return http.StatusBadRequest
}

// ClientFault is the client's fault, named by code and answered with its status.
func ClientFault(code refusal.Code, message string) *Failure {
	return &Failure{status: Status(code), origin: originClient, code: string(code), message: message}
}

// Refused is the client's fault a pure core decided.
func Refused(r *refusal.Refusal) *Failure {
	return ClientFault(r.Code, r.Message)
}

// UIFault is the UI server failing on its own. Its message names no detail, which goes to the log.
func UIFault() *Failure {
	return &Failure{status: http.StatusInternalServerError, origin: originUI, code: "internal", message: "the UI server failed"}
}

// DatabaseFault is the database not answering or refusing. A grant refusal names a table and nothing
// the operator can act on, so it maps here rather than to a client fault (ADR-0084).
func DatabaseFault() *Failure {
	return &Failure{status: http.StatusServiceUnavailable, origin: originDatabase, code: "database", message: "the database did not answer"}
}

// ProviderFault is the provider not answering a setup request, checking a client or exchanging a
// consent's code (docs/UI.md section 17.3). Nothing was stored.
func ProviderFault() *Failure {
	return &Failure{status: http.StatusBadGateway, origin: originProvider, code: "provider_unreachable", message: "the provider did not answer"}
}

type errorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the error contract's detail, which every failure's body carries under error, and a
// refused write's body carries beside the problems it names.
type ErrorDetail struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ErrorType is the error contract's declaration.
func ErrorType() schema.Type {
	return schema.Obj("Error", schema.F("error", ErrorDetailType()))
}

// ErrorDetailType is the declaration of the error contract's detail.
func ErrorDetailType() schema.Type {
	return schema.Obj("ErrorDetail",
		schema.F("origin", schema.Str(originClient, originUI, originDatabase, originProvider)),
		schema.F("code", schema.Str()),
		schema.F("message", schema.Str()),
		schema.F("request_id", schema.Str()),
	)
}

// WriteFailure writes f in the one shape every failure has, and records its origin for the request's
// log line.
func WriteFailure(w http.ResponseWriter, r *http.Request, f *Failure) {
	info := RequestLogOf(r.Context())
	info.origin, info.code = f.origin, f.code
	body, err := json.Marshal(errorBody{Error: ErrorDetail{Origin: f.origin, Code: f.code, Message: f.message, RequestID: info.id}})
	if err != nil {
		// The body is plain strings, so this is unreachable. The status still says what happened.
		info.Err = err
		w.WriteHeader(f.status)
		return
	}
	send(w, r, f.status, body)
}

// ClientRefusal records a refusal on the request's log line and returns its detail, for a refused
// write whose body names its problems beside it.
func ClientRefusal(r *http.Request, f *refusal.Refusal) ErrorDetail {
	info := RequestLogOf(r.Context())
	info.origin, info.code = originClient, string(f.Code)
	return ErrorDetail{Origin: originClient, Code: string(f.Code), Message: f.Message, RequestID: info.id}
}

// WriteJSON writes v as a 200 response, or a UI fault when v does not encode.
func WriteJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		RequestLogOf(r.Context()).Err = err
		WriteFailure(w, r, UIFault())
		return
	}
	send(w, r, http.StatusOK, body)
}

// WriteBody writes v as a response of status, or a UI fault when v does not encode.
func WriteBody(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		RequestLogOf(r.Context()).Err = err
		WriteFailure(w, r, UIFault())
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
		RequestLogOf(r.Context()).Err = err
	}
}
