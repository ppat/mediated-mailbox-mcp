package gmail

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// timed is one series of the request latency, its labels and how many requests it observed.
type timed struct {
	Labels string
	Count  uint64
}

// gatheredDurations reads the request latency series back from the registry.
func gatheredDurations(t *testing.T, reg *prometheus.Registry) []timed {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var out []timed
	for _, f := range families {
		if f.GetName() != durationSeries {
			continue
		}
		for _, m := range f.GetMetric() {
			var labels []string
			for _, l := range m.GetLabel() {
				labels = append(labels, l.GetName()+"="+l.GetValue())
			}
			out = append(out, timed{Labels: strings.Join(labels, ","), Count: m.GetHistogram().GetSampleCount()})
		}
	}
	return out
}

// Every request the adapter sends is timed under the Gmail method it calls, its provider and its
// outcome. Each port call here sends one request that fails, which for an enumeration is its read of
// the profile.
func TestEveryRequestSentIsTimedByItsEndpoint(t *testing.T) {
	cases := []struct {
		name, endpoint string
		call           func(context.Context, *Adapter) error
	}{
		{"a label listing", "labels.list", func(ctx context.Context, a *Adapter) error { _, err := a.ListLabels(ctx); return err }},
		{"a body", "messages.get", func(ctx context.Context, a *Adapter) error {
			_, err := a.GetMessageBody(ctx, "18c2f0a1b2c3d4e5")
			return err
		}},
		{"a thread", "threads.get", func(ctx context.Context, a *Adapter) error {
			_, err := a.GetThreadMetadata(ctx, "18c2f0a1b2c3d4e0")
			return err
		}},
		{"a cursor", "getProfile", func(ctx context.Context, a *Adapter) error { _, err := a.CurrentCursor(ctx); return err }},
		{"changes", "history.list", func(ctx context.Context, a *Adapter) error { _, err := a.ChangesSince(ctx, cursor(12)); return err }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, reg, ctx := testAdapter(t)
			if err := c.call(ctx, a); !errors.Is(err, context.Canceled) {
				t.Fatalf("the call returned %v, want the request to fail on the cancelled context", err)
			}
			want := []timed{{Labels: "endpoint=" + c.endpoint + ",outcome=failed,provider=gmail", Count: 1}}
			if diff := cmp.Diff(want, gatheredDurations(t, reg), compare.Options); diff != "" {
				t.Errorf("request latency observed (-want +got):\n%s", diff)
			}
		})
	}
}

// Every request names the Gmail method it calls, so no request's latency goes unlabelled.
func TestEveryRequestNamesItsEndpoint(t *testing.T) {
	create, err := createLabelRequest("Projects/Alpha")
	if err != nil {
		t.Fatal(err)
	}
	modify, err := modifyRequest("18c2f0a1b2c3d4e5", []string{"Label_1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for name, r := range map[string]request{
		"labels": labelsRequest(), "create": create, "threads": threadsRequest(search{}, "", threadsPerPage),
		"messages": messagesRequest("", 100), "metadata": metadataRequest("m1"), "message": messageRequest("m1", formatFull),
		"thread": threadRequest("t1"), "modify": modify, "profile": profileRequest(), "history": historyRequest(12, 100),
	} {
		got[name] = r.Endpoint
	}
	want := map[string]string{
		"labels": "labels.list", "create": "labels.create", "threads": "threads.list", "messages": "messages.list",
		"metadata": "messages.get", "message": "messages.get", "thread": "threads.get", "modify": "messages.modify",
		"profile": "getProfile", "history": "history.list",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the requests' endpoints (-want +got):\n%s", diff)
	}
}

// A request's outcome is throttled when the provider throttled it, failed for any other error, and
// ok without one.
func TestARequestsOutcomeIsTold(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "ok"},
		{parseError(429, "", time.Now(), nil), "throttled"},
		{parseError(403, "", time.Now(), []byte(`{"error":{"errors":[{"reason":"dailyLimitExceeded"}]}}`)), "throttled"},
		{parseError(500, "", time.Now(), nil), "failed"},
		{fmt.Errorf("gmail: sending a request: %w", mail.ErrProvider), "failed"},
		{context.DeadlineExceeded, "failed"},
	}
	for _, c := range cases {
		if got := outcomeOf(c.err); got != c.want {
			t.Errorf("outcomeOf(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
