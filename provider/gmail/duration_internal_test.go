package gmail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// staticTokens hands out one access token.
type staticTokens struct{}

func (staticTokens) AccessToken(context.Context) (string, error) { return "ya29.test", nil }

// A request is timed for as long as its exchange takes. The client's proxy holds each connection for
// a while and then closes it without answering, so the request fails after that wait and never
// reaches Google (ADR-0043), and its observation is at least the wait.
func TestARequestIsTimedForItsExchange(t *testing.T) {
	const hold = 50 * time.Millisecond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Error(err)
		}
	})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			time.Sleep(hold)
			if err := conn.Close(); err != nil {
				return
			}
		}
	}()
	proxy := &url.URL{Scheme: "http", Host: ln.Addr().String()}
	reg := prometheus.NewRegistry()
	metrics, err := NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Config{
		Account: "you@example.com", Client: &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}},
		Tokens: staticTokens{}, Metrics: metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ListLabels(t.Context()); err == nil {
		t.Fatal("a request through a proxy that answers nothing succeeded")
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	var count uint64
	for _, f := range families {
		if f.GetName() == durationSeries {
			for _, m := range f.GetMetric() {
				sum += m.GetHistogram().GetSampleSum()
				count += m.GetHistogram().GetSampleCount()
			}
		}
	}
	if count != 1 || sum < hold.Seconds() {
		t.Errorf("the request was observed %d times for %v seconds, want once for at least %v", count, sum, hold.Seconds())
	}
}
