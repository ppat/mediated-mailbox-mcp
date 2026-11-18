package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/failure"
)

// Origin is where a failed call failed, one of three a client tells apart (ADR-0101). It and the
// names below are the failure core's, named here so the roots, which reach no lower than this layer,
// can read them.
type Origin = failure.Origin

// The three origins.
const (
	OriginClient   = failure.OriginClient
	OriginAPI      = failure.OriginAPI
	OriginProvider = failure.OriginProvider
)

// ArgumentError is an operation's refusal of an argument the client sent (ADR-0033).
type ArgumentError = failure.ArgumentError

// Refuse returns an ArgumentError with message.
func Refuse(message string) *ArgumentError { return failure.Refuse(message) }

// The refusals of a call, each the client's.
var (
	ErrMissingAccount   = failure.ErrMissingAccount
	ErrUnknownAccount   = failure.ErrUnknownAccount
	ErrAmbiguousAccount = failure.ErrAmbiguousAccount
	ErrRepeatedArgument = failure.ErrRepeatedArgument
	ErrTooManyArguments = failure.ErrTooManyArguments
	ErrNotAnObject      = failure.ErrNotAnObject
	ErrNoOperation      = failure.ErrNoOperation
)

// FromProvider returns err, an error a provider call returned, as the provider's failure when the
// provider's answer caused it, and unchanged otherwise (ADR-0101).
func FromProvider(err error) error { return failure.FromProvider(err) }

// Classify returns the origin of err, an error a call returned, and the message a client receives.
func Classify(err error) (Origin, string) { return failure.Classify(err) }

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

// Failed logs the failed call of the operation named name through logger, at the level its origin
// sets, and returns the content the call answers with, so both roots log and answer a failure alike
// (ADR-0053, ADR-0101, ADR-0122).
func Failed(ctx context.Context, logger *slog.Logger, name string, err error) json.RawMessage {
	logger.Log(ctx, FailureLevel(err), "operation failed", "operation", name, "error", err)
	return Failure(err)
}

// Failure returns the structured content a failed call answers with on both roots, its origin and
// its message (ADR-0101).
func Failure(err error) json.RawMessage {
	origin, message := Classify(err)
	out, merr := json.Marshal(failureContent{Error: failureBody{Origin: origin, Message: message}})
	if merr != nil {
		return json.RawMessage(`{"error":{"origin":"api","message":"` + failure.APIServerMessage + `"}}`)
	}
	return out
}

// FailureSchema is the JSON Schema of a failure's content, which the contract document carries.
const FailureSchema = `{"type":"object","properties":{"error":{"type":"object","properties":{` +
	`"origin":{"type":"string","enum":["client","api","provider"]},"message":{"type":"string"}},` +
	`"required":["origin","message"]}},"required":["error"]}`

type failureContent struct {
	Error failureBody `json:"error"`
}

type failureBody struct {
	Origin  Origin `json:"origin"`
	Message string `json:"message"`
}
