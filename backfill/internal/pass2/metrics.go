package pass2

import "github.com/prometheus/client_golang/prometheus"

// backlogName is the series of the scan backlog's depth, a watched metric because a growing backlog
// turns messages into body denials that read like permission failures (ADR-0093, O2).
const backlogName = "mediated_mailbox_backfill_scan_backlog"

// Metrics are pass 2's series in one process (ADR-0076).
type Metrics struct {
	backlog *prometheus.GaugeVec
}

// NewMetrics registers pass 2's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{backlog: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: backlogName,
		Help: "The account's messages waiting for their content scan, read after each step of backfill's second pass.",
	}, []string{"account"})}
	if err := reg.Register(m.backlog); err != nil {
		return nil, err
	}
	return m, nil
}

// Backlog sets the account's series to the number of its messages waiting for a scan.
func (m *Metrics) Backlog(account string, pending int64) {
	m.backlog.WithLabelValues(account).Set(float64(pending))
}
