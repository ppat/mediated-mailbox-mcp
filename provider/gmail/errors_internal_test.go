package gmail

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// gmailErrorBody is an error response in the shape Google's error guide documents.
func gmailErrorBody(code, reason, message string) string {
	return `{"error": {"code": ` + code + `, "message": "` + message + `", "errors": [{"message": "` + message + `", "domain": "usageLimits", "reason": "` + reason + `"}], "status": "X"}}`
}

var errorTime = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

func TestParseErrorMapsToThePortsErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"unknown message", 404, gmailErrorBody("404", "notFound", "Requested entity was not found."), mail.ErrNotFound},
		{"identifier Gmail cannot read", 400, gmailErrorBody("400", "invalidArgument", "Invalid id value"), mail.ErrNotFound},
		{"bad request", 400, gmailErrorBody("400", "badRequest", "Invalid label name"), mail.ErrInvalid},
		{"conflict", 409, gmailErrorBody("409", "conflict", "Label name exists or conflicts"), mail.ErrInvalid},
		{"expired token", 401, gmailErrorBody("401", "authError", "Invalid Credentials"), mail.ErrAuthentication},
		{"missing scope", 403, gmailErrorBody("403", "insufficientPermissions", "Request had insufficient authentication scopes."), mail.ErrAuthentication},
		{"server error", 500, gmailErrorBody("500", "backendError", "Backend Error"), mail.ErrProvider},
		{"unavailable without a body", 503, "", mail.ErrProvider},
		{"not JSON", 502, "<html>Bad Gateway</html>", mail.ErrProvider},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := parseError(c.status, "", errorTime, []byte(c.body))
			if !errors.Is(err, c.want) {
				t.Errorf("parseError returned %v, want an error wrapping %v", err, c.want)
			}
			if _, ok := mail.Throttled(err); ok {
				t.Errorf("parseError returned a throttle for %s", c.name)
			}
		})
	}
}

// A throttle carries whose limit was reached, as Google's error guide states it for each reason,
// and the wait Gmail asked for.
func TestParseErrorReadsAThrottle(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		retryAfter string
		body       string
		want       mail.ThrottleSignal
	}{
		{"too many requests", 429, "", gmailErrorBody("429", "rateLimitExceeded", "Too many requests"), mail.ThrottleSignal{Scope: mail.ScopePerUser}},
		{"too many requests without a body", 429, "7", "", mail.ThrottleSignal{Scope: mail.ScopePerUser, RetryAfterMillis: 7000, HasRetryAfter: true}},
		{"user rate limit", 403, "", gmailErrorBody("403", "userRateLimitExceeded", "User Rate Limit Exceeded"), mail.ThrottleSignal{Scope: mail.ScopePerUser}},
		{"rate limit", 403, "", gmailErrorBody("403", "rateLimitExceeded", "Rate Limit Exceeded"), mail.ThrottleSignal{Scope: mail.ScopePerUser}},
		{
			"daily project limit, with a date to retry after",
			403, "Thu, 24 Sep 2026 12:00:30 GMT",
			gmailErrorBody("403", "dailyLimitExceeded", "Daily Limit Exceeded"),
			mail.ThrottleSignal{Scope: mail.ScopePerProject, RetryAfterMillis: 30000, HasRetryAfter: true},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := parseError(c.status, c.retryAfter, errorTime, []byte(c.body))
			if !errors.Is(err, mail.ErrThrottled) {
				t.Fatalf("parseError returned %v, want an error wrapping %v", err, mail.ErrThrottled)
			}
			got, _ := mail.Throttled(err)
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("throttle signal (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRetryAfterMillis(t *testing.T) {
	cases := []struct {
		value string
		ms    int64
		ok    bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{" 12 ", 12000, true},
		{"-3", 0, false},
		{"Thu, 24 Sep 2026 12:00:01 GMT", 1000, true},
		{"Thu, 24 Sep 2026 11:59:00 GMT", 0, true},
		{"soon", 0, false},
	}
	for _, c := range cases {
		ms, ok := retryAfterMillis(c.value, errorTime)
		if ms != c.ms || ok != c.ok {
			t.Errorf("retryAfterMillis(%q) = %d, %v, want %d, %v", c.value, ms, ok, c.ms, c.ok)
		}
	}
}
