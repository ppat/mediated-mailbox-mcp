package provider_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The series the runaway rule matches by name, and the latency series no rule reads, written out
// rather than read from the package (ADR-0077, ADR-0125).
const (
	costSeries     = "mediated_mailbox_provider_request_cost_total"
	hardCapSeries  = "mediated_mailbox_provider_hard_cap"
	durationSeries = "mediated_mailbox_provider_request_duration_seconds"
)

// series is one series the registry holds, its name, its labels and its value, a histogram's value
// being its count of observations.
type series struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// gathered reads every series back from reg.
func gathered(t *testing.T, reg *prometheus.Registry) []series {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var out []series
	for _, f := range families {
		for _, m := range f.GetMetric() {
			s := series{Name: f.GetName(), Labels: map[string]string{}}
			s.Value = m.GetCounter().GetValue() + m.GetGauge().GetValue() + float64(m.GetHistogram().GetSampleCount())
			for _, l := range m.GetLabel() {
				s.Labels[l.GetName()] = l.GetValue()
			}
			out = append(out, s)
		}
	}
	return out
}

// cancellingTokens hands out a token and cancels the call's context as it does, so the request that
// follows is built, counted and handed to a real HTTP client that fails it without reaching the
// network. There is no stand-in for Google (ADR-0043), so this is how far a request goes here.
type cancellingTokens struct{ cancel context.CancelFunc }

func (c cancellingTokens) AccessToken(context.Context) (string, error) {
	c.cancel()
	return "ya29.test", nil
}

// Every request the Gmail adapter sends is counted at what Gmail charges for it, under the account and
// the gmail provider label, whether or not it succeeds, and the account's hard cap, 80% of Gmail's
// declared 100 units a second, is emitted beside the count. Each request is timed under the provider,
// the Gmail method it calls and its outcome, and carries no account (ADR-0077, ADR-0125). The two
// requests here are a label listing, one unit, and a body, twenty, each failing once it is sent.
func TestEveryRequestTheAdapterSendsIsOnTheSharedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	shared, err := provider.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := shared.For("gmail")
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func(context.Context, *gmail.Adapter) error{
		func(ctx context.Context, a *gmail.Adapter) error { _, err := a.ListLabels(ctx); return err },
		func(ctx context.Context, a *gmail.Adapter) error {
			_, err := a.GetMessageBody(ctx, "18c2f0a1b2c3d4e5", "")
			return err
		},
	} {
		ctx, cancel := context.WithCancel(t.Context())
		a, err := gmail.New(gmail.Config{Account: "you@example.com", Client: &http.Client{}, Tokens: cancellingTokens{cancel}, Metrics: requests})
		if err != nil {
			t.Fatal(err)
		}
		if err := call(ctx, a); !errors.Is(err, context.Canceled) {
			t.Fatalf("the call returned %v, want the request to fail on the cancelled context", err)
		}
		cancel()
	}
	account := map[string]string{"account": "you@example.com", "provider": "gmail"}
	want := []series{
		{Name: hardCapSeries, Labels: account, Value: 80},
		{Name: costSeries, Labels: account, Value: 21},
		{Name: durationSeries, Labels: map[string]string{"provider": "gmail", "endpoint": "labels.list", "outcome": "failed"}, Value: 1},
		{Name: durationSeries, Labels: map[string]string{"provider": "gmail", "endpoint": "messages.get", "outcome": "failed"}, Value: 1},
	}
	if diff := cmp.Diff(want, gathered(t, reg), compare.Options, sortedSeries); diff != "" {
		t.Errorf("the series (-want +got):\n%s", diff)
	}
}

// sortedSeries compares two lists of series as sets.
var sortedSeries = cmp.Transformer("sorted", func(in []series) map[string]series {
	out := map[string]series{}
	for _, s := range in {
		out[s.Name+"|"+s.Labels["account"]+"|"+s.Labels["endpoint"]] = s
	}
	return out
})

// A provider no shipped adapter serves has no series, since it has no ceiling for the hard cap.
func TestAProviderWithNoAdapterHasNoSeries(t *testing.T) {
	shared, err := provider.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shared.For("jmap"); err == nil {
		t.Error("For returned the series of a provider no adapter serves")
	}
}

// The series are registered once on a registerer, so a second set on the same registerer is refused
// rather than counting the same requests twice.
func TestTheSeriesRegisterOncePerRegisterer(t *testing.T) {
	reg := prometheus.NewRegistry()
	if _, err := provider.NewMetrics(reg); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.NewMetrics(reg); err == nil {
		t.Error("a second NewMetrics on the same registerer succeeded")
	}
}
