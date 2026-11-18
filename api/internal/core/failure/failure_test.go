package failure_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/failure"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// told is what a client is told of a failure.
type told struct {
	Origin  failure.Origin
	Message string
}

// Every failure is the client's, the provider's or the API server's, with the message its kind
// carries, written out here rather than read from the package. An argument refusal and the
// refusals of a call name what the client sent, a provider's failure names its kind and never the
// provider's text, and every other error says only that the call failed inside the API server, never
// its own text (ADR-0101).
func TestClassify(t *testing.T) {
	inside := "the operation failed inside the API server; get_system_status reports the account's operational state"
	cases := map[string]struct {
		err  error
		want told
	}{
		"an argument refusal":        {errors.Join(errors.New("reading"), failure.Refuse("cursor is not a cursor this listing returned")), told{"client", "cursor is not a cursor this listing returned"}},
		"a missing account":          {failure.ErrMissingAccount, told{"client", "the operation needs account_id, and the call carries none"}},
		"an unknown account":         {failure.ErrUnknownAccount, told{"client", "the operation's account_id names no account the API server serves"}},
		"an ambiguous account":       {failure.ErrAmbiguousAccount, told{"client", "the call's arguments name the account more than once"}},
		"a repeated argument":        {failure.ErrRepeatedArgument, told{"client", "the call's arguments name an argument more than once"}},
		"too many arguments":         {failure.ErrTooManyArguments, told{"client", "the call's arguments hold more keys than any operation takes"}},
		"arguments not an object":    {failure.ErrNotAnObject, told{"client", "the call's arguments are not a JSON object"}},
		"no operation":               {failure.ErrNoOperation, told{"client", "no such operation"}},
		"a throttle":                 {failure.FromProvider(errors.Join(errors.New("gmail said 429"), mail.ErrThrottled)), told{"provider", "the provider throttled the request; get_system_status reports the rate controller's state"}},
		"a refused credential":       {failure.FromProvider(errors.Join(errors.New("gmail said 401"), mail.ErrAuthentication)), told{"provider", "the provider refused the account's credential; get_system_status reports the last authentication"}},
		"a message gone":             {failure.FromProvider(errors.Join(errors.New("gmail said 404"), mail.ErrNotFound)), told{"provider", "the provider no longer holds the message"}},
		"a provider failure":         {failure.FromProvider(errors.Join(errors.New("gmail said 503"), mail.ErrProvider)), told{"provider", "the provider failed the request or did not answer"}},
		"a database failure":         {failure.FromProvider(errors.New("the pool is closed: secret detail")), told{"api", inside}},
		"a port error the API built": {failure.FromProvider(errors.Join(errors.New("bad id"), mail.ErrInvalid)), told{"api", inside}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			origin, message := failure.Classify(c.err)
			if diff := cmp.Diff(c.want, told{origin, message}, compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// A provider's failure keeps the error it wraps, so the shell's log carries the cause the client is
// not told.
func TestAProvidersFailureKeepsItsCause(t *testing.T) {
	cause := errors.Join(errors.New("gmail said 503"), mail.ErrProvider)
	if err := failure.FromProvider(cause); !errors.Is(err, mail.ErrProvider) || !errors.Is(err, cause) {
		t.Errorf("FromProvider(%v) = %v, which no longer wraps its cause", cause, err)
	}
}
