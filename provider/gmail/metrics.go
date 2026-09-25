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
	requestCost *prometheus.CounterVec
	hardCap     *prometheus.GaugeVec
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
	}
	for _, c := range []prometheus.Collector{m.requestCost, m.hardCap} {
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
