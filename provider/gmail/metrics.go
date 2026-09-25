package gmail

import (
	"github.com/prometheus/client_golang/prometheus"
)

// requestCostName is the counter the runaway rule reads, in packaging/chart/alerting-rules.yaml
// (ADR-0077). The rule matches its name and its provider label, so both stay exactly as written.
const requestCostName = "mediated_mailbox_provider_request_cost_total"

// providerLabel is this adapter's value of the counter's provider label.
const providerLabel = "gmail"

// Metrics are the adapter's series in one process (ADR-0076). A process builds them once on its own
// registry and passes them to every Adapter it builds, one per account.
type Metrics struct {
	requestCost *prometheus.CounterVec
}

// NewMetrics registers the adapter's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		requestCost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: requestCostName,
			Help: "The cost of every provider request this process sent, in the provider's units, failed requests included.",
		}, []string{"account", "provider"}),
	}
	if err := reg.Register(m.requestCost); err != nil {
		return nil, err
	}
	return m, nil
}

// countRequest adds one request's cost to the account's count.
func (m *Metrics) countRequest(account string, units int) {
	m.requestCost.WithLabelValues(account, providerLabel).Add(float64(units))
}
