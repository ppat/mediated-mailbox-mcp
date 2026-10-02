package index

import (
	"errors"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// ErrorClass is a failure's error class, as a run's failed items record it (ADR-0016).
type ErrorClass string

// The error classes of a failure the provider reported, and NotProvider for one it did not.
const (
	NotProvider    ErrorClass = ""
	Throttled      ErrorClass = "throttled"
	ProviderError  ErrorClass = "provider_error"
	Authentication ErrorClass = "authentication"
	Gone           ErrorClass = "gone"
	Validation     ErrorClass = "validation"
)

// ClassOf returns the error class of a failure the provider reported, by the Provider Port's error it
// wraps, and NotProvider for any other failure, such as the rate limiter failing to record a call.
func ClassOf(err error) ErrorClass {
	switch {
	case errors.Is(err, mail.ErrThrottled):
		return Throttled
	case errors.Is(err, mail.ErrProvider):
		return ProviderError
	case errors.Is(err, mail.ErrAuthentication):
		return Authentication
	case errors.Is(err, mail.ErrNotFound):
		return Gone
	case errors.Is(err, mail.ErrInvalid):
		return Validation
	default:
		return NotProvider
	}
}
