package gmail

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// cancellingTokens hands out a token and cancels the call's context as it does, so the request that
// follows is built, counted and handed to a real HTTP client that fails it without reaching the
// network. There is no stand-in for Google (ADR-0043), so this is how far a request goes here.
type cancellingTokens struct{ cancel context.CancelFunc }

func (c cancellingTokens) AccessToken(context.Context) (string, error) {
	c.cancel()
	return "ya29.test", nil
}

// refusingTokens fails every token request, so no request is ever sent.
type refusingTokens struct{}

func (refusingTokens) AccessToken(context.Context) (string, error) {
	return "", errors.New("invalid_grant")
}

// counted is one series of the request cost counter or of the hard cap.
type counted struct {
	Account  string
	Provider string
	Value    float64
}

// The two series the adapter emits, by the names the runaway rule matches.
const (
	costSeries    = "mediated_mailbox_provider_request_cost_total"
	hardCapSeries = "mediated_mailbox_provider_hard_cap"
)

// gathered reads the series named name back from the registry, with the labels the runaway rule
// matches on. A series the adapter does not emit, or a label the rule does not expect, fails the
// test.
func gathered(t *testing.T, reg *prometheus.Registry, name string) []counted {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var out []counted
	for _, f := range families {
		if f.GetName() != costSeries && f.GetName() != hardCapSeries {
			t.Errorf("the registry holds a series named %q", f.GetName())
			continue
		}
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			c := counted{Value: m.GetCounter().GetValue() + m.GetGauge().GetValue()}
			for _, l := range m.GetLabel() {
				switch l.GetName() {
				case "account":
					c.Account = l.GetValue()
				case "provider":
					c.Provider = l.GetValue()
				default:
					t.Errorf("the series carries a label named %q", l.GetName())
				}
			}
			out = append(out, c)
		}
	}
	return out
}

// gatheredCost reads the request cost counter back from the registry.
func gatheredCost(t *testing.T, reg *prometheus.Registry) []counted {
	t.Helper()
	return gathered(t, reg, costSeries)
}

// testAdapter returns an adapter whose every request fails once it is counted, with the registry its
// counter is on and the context its calls take.
func testAdapter(t *testing.T) (*Adapter, *prometheus.Registry, context.Context) {
	t.Helper()
	reg := prometheus.NewRegistry()
	metrics, err := NewMetrics(reg)
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	a, err := New(Config{Account: "you@example.com", Client: &http.Client{}, Tokens: cancellingTokens{cancel}, Metrics: metrics})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a, reg, ctx
}

// Every request the adapter sends is counted at what Gmail charges for it, under the account and
// the gmail provider label, whether or not it succeeds, and nothing about a lease enters into it.
// The account's hard cap is emitted beside the count (ADR-0077). Each port call here sends one
// request that fails.
func TestEveryRequestSentIsCounted(t *testing.T) {
	cases := []struct {
		name  string
		call  func(context.Context, *Adapter) error
		units float64
	}{
		{"a label listing", func(ctx context.Context, a *Adapter) error { _, err := a.ListLabels(ctx); return err }, 1},
		{"a body", func(ctx context.Context, a *Adapter) error {
			_, err := a.GetMessageBody(ctx, "18c2f0a1b2c3d4e5")
			return err
		}, 20},
		{"a thread", func(ctx context.Context, a *Adapter) error {
			_, err := a.GetThreadMetadata(ctx, "18c2f0a1b2c3d4e0")
			return err
		}, 40},
		{"a cursor", func(ctx context.Context, a *Adapter) error { _, err := a.CurrentCursor(ctx); return err }, 1},
		{"changes", func(ctx context.Context, a *Adapter) error { _, err := a.ChangesSince(ctx, cursor(12)); return err }, 2},
		{"an enumeration", func(ctx context.Context, a *Adapter) error { _, err := a.EnumerateAll(ctx, ""); return err }, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, reg, ctx := testAdapter(t)
			if err := c.call(ctx, a); !errors.Is(err, context.Canceled) {
				t.Fatalf("the call returned %v, want the request to fail on the cancelled context", err)
			}
			want := []counted{{Account: "you@example.com", Provider: "gmail", Value: c.units}}
			if diff := cmp.Diff(want, gatheredCost(t, reg), compare.Options); diff != "" {
				t.Errorf("request cost counted (-want +got):\n%s", diff)
			}
			// The hard cap beside the count is 80% of Gmail's declared 100 units a second.
			wantCap := []counted{{Account: "you@example.com", Provider: "gmail", Value: 80}}
			if diff := cmp.Diff(wantCap, gathered(t, reg, hardCapSeries), compare.Options); diff != "" {
				t.Errorf("hard cap emitted (-want +got):\n%s", diff)
			}
		})
	}
}

// A request that never leaves, because no access token was had, is not counted, and sets no hard
// cap.
func TestARequestNeverSentIsNotCounted(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics, err := NewMetrics(reg)
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	a, err := New(Config{Account: "you@example.com", Client: &http.Client{}, Tokens: refusingTokens{}, Metrics: metrics})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := a.ListLabels(t.Context()); !errors.Is(err, mail.ErrAuthentication) {
		t.Errorf("ListLabels returned %v, want an error wrapping %v", err, mail.ErrAuthentication)
	}
	if got := gatheredCost(t, reg); len(got) != 0 {
		t.Errorf("request cost counted %+v, want nothing", got)
	}
	if got := gathered(t, reg, hardCapSeries); len(got) != 0 {
		t.Errorf("hard cap emitted %+v, want nothing", got)
	}
}

// A call sends a request only while it stays within the cost the rate profile declares for the call,
// counting each request it sent, and refuses the one that would take it past without sending it.
func TestACallIsHeldToItsDeclaredCost(t *testing.T) {
	a, reg, ctx := testAdapter(t)
	c := a.begin(mail.OpGetThreadMetadata, 0)
	if c.declared != 41 {
		t.Fatalf("a thread read declares %d units, want 41", c.declared)
	}
	for _, r := range []request{threadRequest("18c2f0a1b2c3d4e0"), labelsRequest()} {
		if _, err := c.do(ctx, r); !errors.Is(err, context.Canceled) {
			t.Fatalf("a request within the declared cost returned %v, want it sent", err)
		}
	}
	if _, err := c.do(ctx, labelsRequest()); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("a request past the declared cost returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
	want := []counted{{Account: "you@example.com", Provider: "gmail", Value: 41}}
	if diff := cmp.Diff(want, gatheredCost(t, reg), compare.Options); diff != "" {
		t.Errorf("request cost counted (-want +got):\n%s", diff)
	}
}

func TestWithin(t *testing.T) {
	cases := []struct {
		spent, units, declared int
		want                   bool
	}{
		{0, 40, 41, true},
		{40, 1, 41, true},
		{41, 1, 41, false},
		{40, 2, 41, false},
		{0, 0, 0, true},
		{0, 1, 0, false},
	}
	for _, c := range cases {
		if got := within(c.spent, c.units, c.declared); got != c.want {
			t.Errorf("within(%d, %d, %d) = %v, want %v", c.spent, c.units, c.declared, got, c.want)
		}
	}
}

// The dearest sequence of requests each operation can send fits the cost the rate profile declares
// for it, so the hold on a call never refuses a request the operation needs. Each sequence is the
// operation's longest path through its method.
func TestEachOperationsDearestPathFitsItsCall(t *testing.T) {
	create := must[request](t)(createLabelRequest("project"))
	modify := must[request](t)(modifyRequest("m1", nil, []string{"INBOX"}))
	read := messageRequest("m1", formatMinimal)
	get := metadataRequest("m1")
	cases := []struct {
		name     string
		op       mail.Operation
		messages int
		path     []request
	}{
		{"ListThreads", mail.OpListThreads, 0, []request{labelsRequest(), threadsRequest(search{}, "", threadsPerPage), threadRequest("t1"), labelsRequest()}},
		{"GetThreadMetadata", mail.OpGetThreadMetadata, 0, []request{threadRequest("t1"), labelsRequest()}},
		{"GetMessageMetadata", mail.OpGetMessageMetadata, idsPerMetadataCall, []request{get, get, get, labelsRequest()}},
		{"GetMessageBody", mail.OpGetMessageBody, 1, []request{messageRequest("m1", formatFull)}},
		{"ListLabels", mail.OpListLabels, 0, []request{labelsRequest()}},
		{"EnsureLabel", mail.OpEnsureLabel, 0, []request{labelsRequest(), create, labelsRequest()}},
		{"Mutate, every op changing nothing", mail.OpMutate, opsPerMutation, []request{labelsRequest(), read, read, read}},
		{"Mutate, every op modifying", mail.OpMutate, opsPerMutation, []request{labelsRequest(), modify, modify, modify}},
		{"CurrentCursor", mail.OpCurrentCursor, 0, []request{profileRequest()}},
		{"ChangesSince", mail.OpChangesSince, 0, []request{historyRequest(1, historyPage), labelsRequest()}},
		{"EnumerateAll", mail.OpEnumerateAll, 0, []request{messagesRequest("", messagesPerPage), get, get, get, labelsRequest()}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _, ctx := testAdapter(t)
			call := a.begin(c.op, c.messages)
			for i, r := range c.path {
				if _, err := call.do(ctx, r); errors.Is(err, mail.ErrInvalid) {
					t.Fatalf("request %d, %s %s, was refused after %d of %d units", i, r.Method, r.Path, call.spent, call.declared)
				}
			}
		})
	}
}

// A caller asking for more than one call can cost is refused before anything is sent, so it splits
// its work into calls the rate limiter can issue.
func TestCallerSizedCallsPastTheSecondAreRefused(t *testing.T) {
	a, reg, ctx := testAdapter(t)
	if _, err := a.GetMessageMetadata(ctx, []string{"m1", "m2", "m3", "m4"}); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("GetMessageMetadata of four identifiers returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
	op := must[mail.MutationOp](t)
	ops := []mail.MutationOp{op(mail.StarOp("m1")), op(mail.StarOp("m2")), op(mail.StarOp("m3")), op(mail.StarOp("m4"))}
	if _, err := a.Mutate(ctx, ops); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("Mutate of four ops returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
	if got := gatheredCost(t, reg); len(got) != 0 {
		t.Errorf("request cost counted %+v, want nothing sent", got)
	}
}

// A thread read returns every message of the thread, mapped and oldest first.
func TestParseThread(t *testing.T) {
	messages, err := parseThread([]byte(`{
  "id": "18c2f0a1b2c3d4e0", "historyId": "9876543",
  "messages": [
    {"id": "18c2f0a1b2c3d4e7", "threadId": "18c2f0a1b2c3d4e0", "labelIds": ["INBOX", "Label_13"], "snippet": "re", "sizeEstimate": 10, "internalDate": "1725267600000",
     "payload": {"mimeType": "text/plain", "filename": "", "headers": [{"name": "From", "value": "you@example.com"}, {"name": "Subject", "value": "Re: digest"}], "body": {"size": 2}}},
    {"id": "18c2f0a1b2c3d4e0", "threadId": "18c2f0a1b2c3d4e0", "labelIds": ["Label_13", "UNREAD"], "snippet": "digest", "sizeEstimate": 20, "internalDate": "1725181200000",
     "payload": {"mimeType": "text/plain", "filename": "", "headers": [{"name": "From", "value": "news@example.com"}, {"name": "Subject", "value": "digest"}], "body": {"size": 6}}}
  ]
}`))
	if err != nil {
		t.Fatalf("parseThread: %v", err)
	}
	labels := accountLabels(t)
	mapped := make([]mail.MessageMetadata, len(messages))
	for i, m := range messages {
		mapped[i] = metadata("a", labels, m)
	}
	want := []mail.ThreadMetadata{{AccountID: "a", ID: "18c2f0a1b2c3d4e0", Messages: []mail.MessageMetadata{
		{AccountID: "a", ID: "18c2f0a1b2c3d4e0", ThreadID: "18c2f0a1b2c3d4e0", From: mail.Address{Email: "news@example.com"}, Subject: "digest", Date: 1725181200000, Labels: []string{"reading/digest"}, SizeBytes: 20, Snippet: "digest"},
		{AccountID: "a", ID: "18c2f0a1b2c3d4e7", ThreadID: "18c2f0a1b2c3d4e0", From: mail.Address{Email: "you@example.com"}, Subject: "Re: digest", Date: 1725267600000, Labels: []string{"INBOX", "reading/digest"}, Flags: mail.Flags{Read: true}, SizeBytes: 10, Snippet: "re"},
	}}}
	if diff := cmp.Diff(want, threads("a", []string{"18c2f0a1b2c3d4e0"}, mapped), compare.Options); diff != "" {
		t.Errorf("thread (-want +got):\n%s", diff)
	}
	if _, err := parseThread([]byte(`{"id": "t", "messages": [{"id": "m"}]}`)); !errors.Is(err, mail.ErrProvider) {
		t.Errorf("parseThread of a message with no thread returned %v, want an error wrapping %v", err, mail.ErrProvider)
	}
}
