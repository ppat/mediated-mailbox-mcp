// Package failure holds what a failed call of the client surface tells a client, its origin and its
// message, as a value (ADR-0040, ADR-0101). A failure is the client's, the API server's or the
// provider's, and a client tells the three apart. Its message never carries a cause that could hold
// stored content, a database's text or a provider's own text.
//
// The refusals of a call that the service layer makes before any operation runs sit here with the
// classification, so the errors and the decision of whose they are have one home. The shell logs a
// failure, at the level its origin sets, and writes it as the error contract's JSON.
package failure

import (
	"errors"

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

// APIServerMessage is the one message a failure inside the system carries. It names no cause, which
// could carry stored content or a database's text, and points at the operational state a client can
// read.
const APIServerMessage = "the operation failed inside the API server; get_system_status reports the account's operational state"

// ErrMissingAccount is returned for a call whose arguments carry no account_id string.
var ErrMissingAccount = errors.New("the operation needs account_id, and the call carries none")

// ErrUnknownAccount is returned for a call whose account_id names no account the API server serves.
var ErrUnknownAccount = errors.New("the operation's account_id names no account the API server serves")

// ErrAmbiguousAccount is returned for a call whose arguments hold account_id more than once, or a
// key that differs from account_id only in case. A reader matching keys without regard to case, or
// keeping the last of two, would take a different account from the one checked.
var ErrAmbiguousAccount = errors.New("the call's arguments name the account more than once")

// ErrRepeatedArgument is returned for a call whose arguments hold two top-level keys equal under
// case folding, other than the account's. A reader matching keys without regard to case, or keeping
// the last of two, would take a different argument from the one the other reader took.
var ErrRepeatedArgument = errors.New("the call's arguments name an argument more than once")

// ErrTooManyArguments is returned for a call whose arguments hold more keys than the service layer
// reads.
var ErrTooManyArguments = errors.New("the call's arguments hold more keys than any operation takes")

// ErrNotAnObject is returned for a call whose arguments are not one JSON object.
var ErrNotAnObject = errors.New("the call's arguments are not a JSON object")

// ErrNoOperation is returned for a call naming an operation the registry does not hold.
var ErrNoOperation = errors.New("no such operation")

// ArgumentError is an operation's refusal of an argument the client sent, such as a timestamp that is
// not UTC (ADR-0033). Its message reaches the client, so it names the argument and never echoes
// stored content.
type ArgumentError struct{ message string }

// Refuse returns an ArgumentError with message.
func Refuse(message string) *ArgumentError { return &ArgumentError{message: message} }

func (e *ArgumentError) Error() string { return e.message }

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
// An operation's refusal of an argument and the refusals of a call are the client's, a provider
// call's failure is the provider's, and every other error is the API server's, so a failure nobody
// classified never reads as the client's or the provider's.
func Classify(err error) (Origin, string) {
	var arg *ArgumentError
	if errors.As(err, &arg) {
		return OriginClient, arg.Error()
	}
	for _, refusal := range []error{
		ErrMissingAccount, ErrUnknownAccount, ErrAmbiguousAccount, ErrRepeatedArgument, ErrTooManyArguments,
		ErrNotAnObject, ErrNoOperation,
	} {
		if errors.Is(err, refusal) {
			return OriginClient, refusal.Error()
		}
	}
	var provider *providerError
	if errors.As(err, &provider) {
		return OriginProvider, provider.message
	}
	return OriginAPI, APIServerMessage
}
