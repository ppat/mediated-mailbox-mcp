package gmail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// statusError is a Gmail error response, carrying the port error it maps to.
type statusError struct {
	status  int
	reason  string
	message string
	kind    error
}

func (e statusError) Error() string {
	return fmt.Sprintf("gmail: %d %s: %s: %v", e.status, e.reason, e.message, e.kind)
}

func (e statusError) Unwrap() error { return e.kind }

// invalidID is the message Gmail answers with when a path names an identifier it cannot read.
const invalidID = "Invalid id value"

// parseError maps a response that is not a success to the port's errors. retryAfter is the
// response's Retry-After header, read against now.
func parseError(status int, retryAfter string, now time.Time, body []byte) error {
	var raw struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	// A body that is not Google's error document still has a status to map, with no reason.
	if json.Unmarshal(body, &raw) != nil {
		raw.Error.Message, raw.Error.Errors = "", nil
	}
	e := statusError{status: status, message: raw.Error.Message}
	if len(raw.Error.Errors) > 0 {
		e.reason = raw.Error.Errors[0].Reason
	}
	e.kind = errorKind(e.status, e.reason, e.message, retryAfter, now)
	return e
}

func errorKind(status int, reason, message, retryAfter string, now time.Time) error {
	throttle := func(scope mail.ThrottleScope) error {
		signal := mail.ThrottleSignal{Scope: scope}
		signal.RetryAfterMillis, signal.HasRetryAfter = retryAfterMillis(retryAfter, now)
		return mail.ThrottleError{Signal: signal}
	}
	switch {
	case status == http.StatusTooManyRequests:
		return throttle(mail.ScopePerUser)
	case status == http.StatusForbidden && (reason == "userRateLimitExceeded" || reason == "rateLimitExceeded"):
		return throttle(mail.ScopePerUser)
	case status == http.StatusForbidden && reason == "dailyLimitExceeded":
		return throttle(mail.ScopePerProject)
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return mail.ErrAuthentication
	case status == http.StatusNotFound:
		return mail.ErrNotFound
	case status == http.StatusBadRequest && message == invalidID:
		return mail.ErrNotFound
	case status == http.StatusBadRequest, status == http.StatusConflict:
		return mail.ErrInvalid
	default:
		return mail.ErrProvider
	}
}

// retryAfterMillis reads a Retry-After header, either a number of seconds or a date, as the wait it
// asks for in milliseconds.
func retryAfterMillis(value string, now time.Time) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if s, err := strconv.ParseInt(value, 10, 64); err == nil {
		if s < 0 {
			return 0, false
		}
		return s * 1000, true
	}
	at, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(at.Sub(now).Milliseconds(), 0), true
}
