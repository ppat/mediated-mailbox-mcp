package pass1

import "github.com/prometheus/client_golang/prometheus"

// unclassifiedName is the series counting the messages whose sender the classifier could not
// classify (O2).
const unclassifiedName = "mediated_mailbox_backfill_unclassified_senders_total"

// Metrics are pass 1's series in one process (ADR-0076).
type Metrics struct {
	unclassified *prometheus.CounterVec
}

// NewMetrics registers pass 1's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{unclassified: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: unclassifiedName,
		Help: "The messages made durable in the index whose sender the classifier could not classify, which classifies them restricted.",
	}, []string{"account"})}
	if err := reg.Register(m.unclassified); err != nil {
		return nil, err
	}
	return m, nil
}

// Count adds what one step made durable to the account's series. The series exists for every account
// a run serves, at zero until a sender goes unclassified, so its absence means no run served the
// account.
func (m *Metrics) Count(account string, s Step) {
	m.unclassified.WithLabelValues(account).Add(float64(s.Unclassified))
}
