package mail

import (
	"errors"
	"strconv"
)

// Operation names a port operation, for costing it. The zero value names none.
type Operation uint8

const (
	_ Operation = iota
	// OpListThreads is ListThreads.
	OpListThreads
	// OpGetThreadMetadata is GetThreadMetadata.
	OpGetThreadMetadata
	// OpGetMessageMetadata is GetMessageMetadata.
	OpGetMessageMetadata
	// OpGetMessageBody is GetMessageBody.
	OpGetMessageBody
	// OpListLabels is ListLabels.
	OpListLabels
	// OpEnsureLabel is EnsureLabel.
	OpEnsureLabel
	// OpMutate is Mutate.
	OpMutate
	// OpCurrentCursor is CurrentCursor.
	OpCurrentCursor
	// OpChangesSince is ChangesSince.
	OpChangesSince
	// OpEnumerateAll is EnumerateAll.
	OpEnumerateAll
)

// ProviderOp is one call of a port operation, with the number of messages it names. Messages is the
// number of identifiers a GetMessageMetadata call asks for or the number of ops a Mutate call
// carries, and one for GetMessageBody. It is zero for an operation that names no message up front.
type ProviderOp struct {
	Operation Operation
	Messages  int
}

// OpCost is what one call costs. Weight is in the provider's own units, and OpsCount is the number of
// messages the call covers, for throughput accounting (ADR-0023).
type OpCost struct {
	Weight   float64
	OpsCount int
}

// ThrottleScope is whom a provider throttled. The zero value is unknown.
type ThrottleScope uint8

const (
	// ScopeUnknown is a throttle whose scope the provider did not say.
	ScopeUnknown ThrottleScope = iota
	// ScopePerUser is a throttle on the account.
	ScopePerUser
	// ScopePerProject is a throttle on the application's project, shared by every account.
	ScopePerProject
)

// ThrottleSignal is what a provider said when it throttled a request (ADR-0023).
type ThrottleSignal struct {
	// RetryAfterMillis is how long the provider asked the caller to wait, meaningful only when
	// HasRetryAfter is set. Without it the caller backs off with full jitter (ADR-0024).
	RetryAfterMillis int64
	HasRetryAfter    bool
	Scope            ThrottleScope
}

// ThrottleError is the error a port returns when the provider throttles a request. It wraps
// ErrThrottled.
type ThrottleError struct {
	Signal ThrottleSignal
}

// Error describes the throttle.
func (e ThrottleError) Error() string {
	text := ErrThrottled.Error()
	if e.Signal.HasRetryAfter {
		text += ", retry after " + strconv.FormatInt(e.Signal.RetryAfterMillis, 10) + " ms"
	}
	return text
}

// Unwrap returns ErrThrottled.
func (e ThrottleError) Unwrap() error { return ErrThrottled }

// Throttled returns the signal of the ThrottleError err wraps, and whether it wraps one.
func Throttled(err error) (ThrottleSignal, bool) {
	var t ThrottleError
	if errors.As(err, &t) {
		return t.Signal, true
	}
	return ThrottleSignal{}, false
}

// RateLimitProfile is what an implementation declares about its provider's costs and limits, so
// everything above it sees only a weight and a budget (ADR-0023).
type RateLimitProfile[C any] interface {
	// Cost returns what one call costs.
	Cost(op ProviderOp) OpCost
	// BudgetPerSecond returns the provider's ceiling, in the units Cost weighs in.
	BudgetPerSecond() float64
	// ParseThrottle returns the signal in err when err is a throttle, and whether it is.
	ParseThrottle(err error) (ThrottleSignal, bool)
	// RefreshLimits re-reads limits a provider publishes at run time, and does nothing for a
	// provider whose limits are fixed.
	RefreshLimits(ctx C) error
}
