package gmail

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// The series the runaway rule reads, in packaging/chart/alerting-rules.yaml (ADR-0077). The rule
// matches their names and compares the count against the hard cap per account, so both names stay
// exactly as written.
const (
	requestCostName = "mediated_mailbox_provider_request_cost_total"
	hardCapName     = "mediated_mailbox_provider_hard_cap"
)

// providerLabel is this adapter's value of the series' provider label.
const providerLabel = "gmail"

// Metrics are the adapter's series in one process (ADR-0076). A process builds them once on its own
// registry and passes them to every Adapter it builds, one per account.
type Metrics struct {
	requestCost     *prometheus.CounterVec
	hardCap         *prometheus.GaugeVec
	requestDuration *prometheus.HistogramVec
}

// NewMetrics registers the adapter's series on reg and returns them.
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

// countRequest adds one request's cost to the account's count, and sets the account's hard cap
// beside it, the hard-cap fraction of the ceiling Gmail's rate profile declares. The runaway rule
// compares the one against the other, so it holds no provider's number of its own.
func (m *Metrics) countRequest(account string, units int) {
	m.hardCap.WithLabelValues(account, providerLabel).Set(mail.HardCapFraction * Profile{}.BudgetPerSecond())
	m.requestCost.WithLabelValues(account, providerLabel).Add(float64(units))
}

// requestDurationName is the series timing each request by the Gmail method it calls and how it
// ended, which no rule reads (ADR-0125).
const requestDurationName = "mediated_mailbox_provider_request_duration_seconds"

// observeRequest observes one request's latency in seconds under the Gmail method it called and its
// outcome.
func (m *Metrics) observeRequest(endpoint, outcome string, seconds float64) {
	m.requestDuration.WithLabelValues(providerLabel, endpoint, outcome).Observe(seconds)
}
