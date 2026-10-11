package pass2

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// Metrics are pass 2's series on its job kind's registerer (ADR-0076).
type Metrics struct {
	backlog  *prometheus.GaugeVec
	messages *series.Messages
}

// NewMetrics registers pass 2's series on reg and returns them, counting the messages each page scans
// on messages, which its job kind registered once for every step it runs (ADR-0125). A nil messages
// counts none.
func NewMetrics(reg prometheus.Registerer, messages *series.Messages) (*Metrics, error) {
	backlog, err := series.NewBacklog(reg)
	if err != nil {
		return nil, err
	}
	return &Metrics{backlog: backlog, messages: messages}, nil
}

// Count adds the messages one page scanned and made durable to the account's count.
func (m *Metrics) Count(account string, s Step) {
	m.messages.Add(account, series.StageScanned, s.Scanned)
}

// Backlog sets the account's series to the number of its messages waiting for a scan, read after each
// step of the second pass.
func (m *Metrics) Backlog(account string, pending int64) {
	m.backlog.WithLabelValues(account).Set(float64(pending))
}

// Forget removes the account's series once its second pass has ended, when delta sync's tick emits
// the account's backlog in its place (ADR-0104), or once its job is dropped.
func (m *Metrics) Forget(account string) {
	m.backlog.DeleteLabelValues(account)
}
