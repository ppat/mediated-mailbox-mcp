package provider

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The series the runaway rule reads, in packaging/chart/rules/alerting-rules.yaml (ADR-0077). The rule
// matches their names and compares the count against the hard cap per account, so both names stay
// exactly as written.
const (
	requestCostName = "mediated_mailbox_provider_request_cost_total"
	hardCapName     = "mediated_mailbox_provider_hard_cap"
)

// requestDurationName is the series timing each request by the provider method it calls and how it
// ended, which no rule reads (ADR-0125).
const requestDurationName = "mediated_mailbox_provider_request_duration_seconds"

// Metrics are the shared provider series of one caller, or of one program that builds its adapters
// itself, registered once on its registerer (ADR-0076). Every adapter built over them counts and
// times its requests on them, each under its provider's name.
type Metrics struct {
	requestCost     *prometheus.CounterVec
	hardCap         *prometheus.GaugeVec
	requestDuration *prometheus.HistogramVec
}

// NewMetrics registers the shared provider series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	labels := []string{"account", "provider"}
	m := &Metrics{
		requestCost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: requestCostName,
			Help: "The cost of every provider request this process sent, in the provider's units, failed requests included.",
		}, labels),
		hardCap: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: hardCapName,
			Help: "The account's hard cap, the most the rate limiter issues in any one second, in the provider's units per second, from the ceiling the provider's adapter declares.",
		}, labels),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    requestDurationName,
			Help:    "How long each provider request this process sent took, from sending it to reading its answer, by the provider method it calls and its outcome, ok, throttled or failed.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{"provider", "endpoint", "outcome"}),
	}
	for _, c := range []prometheus.Collector{m.requestCost, m.hardCap, m.requestDuration} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// For returns the series one provider's adapters count and time their requests on, labelled with the
// provider's name, with the account's hard cap set from the ceiling the provider's facts give. A
// provider with no facts has none.
func (m *Metrics) For(name string) (Requests, error) {
	facts, ok := FactsOf(name)
	if !ok {
		return Requests{}, fmt.Errorf("there is no adapter for provider %q", name)
	}
	return Requests{metrics: m, provider: name, ceiling: facts.Ceiling}, nil
}

// Requests are one provider's view of the shared provider series, which its adapter reports every
// request it sends to.
type Requests struct {
	metrics  *Metrics
	provider string
	ceiling  float64
}

// Sent adds one request's cost to the account's count, and sets the account's hard cap beside it,
// the hard-cap fraction of the ceiling the provider's rate profile declares. The runaway rule
// compares the one against the other, so it holds no provider's number of its own.
func (r Requests) Sent(account string, units int) {
	r.metrics.hardCap.WithLabelValues(account, r.provider).Set(mail.HardCapFraction * r.ceiling)
	r.metrics.requestCost.WithLabelValues(account, r.provider).Add(float64(units))
}

// Answered observes one request's latency in seconds under the provider method it called and its
// outcome.
func (r Requests) Answered(endpoint, outcome string, seconds float64) {
	r.metrics.requestDuration.WithLabelValues(r.provider, endpoint, outcome).Observe(seconds)
}
