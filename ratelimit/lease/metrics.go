package lease

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
)

// The per-process series every spending process emits through its Limiter, each labelled by account.
// ratelimit/README.md lists them with the account-level series and the alerting rules that read them.
const (
	rateName      = "mediated_mailbox_ratelimit_rate"
	grantedName   = "mediated_mailbox_ratelimit_granted_total"
	throttlesName = "mediated_mailbox_ratelimit_throttles_total"
)

// Metrics are one process's rate, lease and throttle series (ADR-0024, ADR-0076). A process builds
// them once on its own registry and passes them to its Limiter.
type Metrics struct {
	rate      *prometheus.GaugeVec
	granted   *prometheus.CounterVec
	throttles *prometheus.CounterVec
}

// NewMetrics registers the series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		rate: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: rateName,
			Help: "The account's current rate as this process last read or set it, in the provider's units per second.",
		}, []string{"account"}),
		granted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: grantedName,
			Help: "Tokens leased to this process, in the provider's units, by priority class.",
		}, []string{"account", "class"}),
		throttles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: throttlesName,
			Help: "Throttles this process received from the provider, by whom the provider throttled.",
		}, []string{"account", "scope"}),
	}
	for _, c := range []prometheus.Collector{m.rate, m.granted, m.throttles} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Metrics) observeRate(account string, rate float64) {
	if m != nil {
		m.rate.WithLabelValues(account).Set(rate)
	}
}

func (m *Metrics) observeGrant(account string, lease core.Lease) {
	if m != nil {
		m.granted.WithLabelValues(account, className(lease.Class)).Add(lease.Tokens)
	}
}

func (m *Metrics) observeThrottle(account string, scope mail.ThrottleScope) {
	if m != nil {
		m.throttles.WithLabelValues(account, scopeName(scope)).Inc()
	}
}

func scopeName(s mail.ThrottleScope) string {
	switch s {
	case mail.ScopePerUser:
		return "user"
	case mail.ScopePerProject:
		return "project"
	case mail.ScopeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}
