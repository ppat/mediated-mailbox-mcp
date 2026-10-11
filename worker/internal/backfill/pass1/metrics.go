package pass1

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// Metrics are pass 1's series on its job kind's registerer (ADR-0076).
type Metrics struct {
	unclassified *prometheus.CounterVec
	messages     *series.Messages
}

// NewMetrics registers pass 1's series on reg and returns them, counting the messages each step adds
// to the index on messages, which its job kind registered once for every step it runs (ADR-0125). A nil
// messages counts none.
func NewMetrics(reg prometheus.Registerer, messages *series.Messages) (*Metrics, error) {
	unclassified, err := series.NewUnclassified(reg)
	if err != nil {
		return nil, err
	}
	return &Metrics{unclassified: unclassified, messages: messages}, nil
}

// Count adds what one step made durable to the account's series. The unclassified series exists for
// every account a run serves, at zero until a sender goes unclassified, so its absence means no run
// served the account.
func (m *Metrics) Count(account string, s Step) {
	m.unclassified.WithLabelValues(account).Add(float64(s.Unclassified))
	m.messages.Add(account, series.StageIndexed, s.Added)
}

// Forget removes the account's series once its job is dropped, since no run serves it any longer.
func (m *Metrics) Forget(account string) {
	m.unclassified.DeleteLabelValues(account)
	m.messages.Forget(account)
}
