package service

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Origin is where a failed call failed, one of three a client tells apart (ADR-0101).
type Origin string

const (
	// OriginClient is a failure of the request itself.
	OriginClient Origin = "client"
	// OriginAPI is a failure inside the system.
	OriginAPI Origin = "api"
	// OriginProvider is a failure in the provider's answer, or its absence.
	OriginProvider Origin = "provider"
)

// apiServerMessage is the one message a failure inside the system carries. It names no cause, which
// could carry stored content or a database's text, and points at the operational state a client can
// read.
const apiServerMessage = "the operation failed inside the API server; get_system_status reports the account's operational state"

// refusals are the registry's refusals of a call, each the client's.
var refusals = []error{
	ErrMissingAccount, ErrUnknownAccount, ErrAmbiguousAccount, ErrRepeatedArgument, ErrTooManyArguments,
	ErrNotAnObject, ErrNoOperation,
}

// providerError is a failure in a provider call, carrying the fixed message for its kind and never
// the provider's own text.
type providerError struct {
	message string
	err     error
}

func (e *providerError) Error() string { return e.message + ": " + e.err.Error() }

func (e *providerError) Unwrap() error { return e.err }

// FromProvider returns err, an error a provider call returned, as the provider's failure when it
// wraps one of the Provider Port's errors that the provider's answer causes, and unchanged otherwise.
// A failure of the rate limiter or the database on the way to the provider stays the API server's, and
// so does a request the port refuses as malformed, which the API server built (ADR-0101, ADR-0010).
func FromProvider(err error) error {
	for _, kind := range []struct {
		err     error
		message string
	}{
		{mail.ErrThrottled, "the provider throttled the request; get_system_status reports the rate controller's state"},
		{mail.ErrAuthentication, "the provider refused the account's credential; get_system_status reports the last authentication"},
		{mail.ErrNotFound, "the provider no longer holds the message"},
		{mail.ErrProvider, "the provider failed the request or did not answer"},
	} {
		if errors.Is(err, kind.err) {
			return &providerError{message: kind.message, err: err}
		}
	}
	return err
}

// Classify returns the origin of err, an error a call returned, and the message a client receives.
// An operation's refusal of an argument and the registry's refusals of a call are the client's, a
// provider call's failure is the provider's, and every other error is the API server's, so a failure
// nobody classified never reads as the client's or the provider's.
func Classify(err error) (Origin, string) {
	var arg *ArgumentError
	if errors.As(err, &arg) {
		return OriginClient, arg.Error()
	}
	for _, refusal := range refusals {
		if errors.Is(err, refusal) {
			return OriginClient, refusal.Error()
		}
	}
	var provider *providerError
	if errors.As(err, &provider) {
		return OriginProvider, provider.message
	}
	return OriginAPI, apiServerMessage
}

// FailureLevel returns the level a failed call is logged at, by its origin, so both roots log a
// failure alike (ADR-0122, ADR-0053). A failure inside the API server is an error an operator acts on.
// A provider's failure, a throttle or a refused credential among them, is a warning, since an
// operator acts on it when it persists. A client's refused request is routine progress, since the
// client is told why and the operator has nothing to act on.
func FailureLevel(err error) slog.Level {
	origin, _ := Classify(err)
	switch origin {
	case OriginClient:
		return slog.LevelInfo
	case OriginProvider:
		return slog.LevelWarn
	case OriginAPI:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}

// Failure returns the structured content a failed call answers with on both roots, its origin and
// its message (ADR-0101).
func Failure(err error) json.RawMessage {
	origin, message := Classify(err)
	out, merr := json.Marshal(failure{Error: failureBody{Origin: origin, Message: message}})
	if merr != nil {
		return json.RawMessage(`{"error":{"origin":"api","message":"` + apiServerMessage + `"}}`)
	}
	return out
}

// FailureSchema is the JSON Schema of a failure's content, which the contract document carries.
const FailureSchema = `{"type":"object","properties":{"error":{"type":"object","properties":{` +
	`"origin":{"type":"string","enum":["client","api","provider"]},"message":{"type":"string"}},` +
	`"required":["origin","message"]}},"required":["error"]}`

type failure struct {
	Error failureBody `json:"error"`
}

type failureBody struct {
	Origin  Origin `json:"origin"`
	Message string `json:"message"`
}
