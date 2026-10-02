package tick

import "github.com/prometheus/client_golang/prometheus"

// The series a tick feeds (ADR-0076). The cursor gap rule in packaging/chart/alerting-rules.yaml reads
// gapsName, so it stays exactly as written (ADR-0077, ADR-0105).
const (
	unclassifiedName = "mediated_mailbox_sync_unclassified_senders_total"
	backlogName      = "mediated_mailbox_sync_scan_backlog"
	gapsName         = "mediated_mailbox_sync_cursor_gaps_total"
)

// Metrics are the tick's series in one process, which runs until stopped, so each keeps its value
// between ticks (ADR-0103).
type Metrics struct {
	unclassified *prometheus.CounterVec
	backlog      *prometheus.GaugeVec
	gaps         *prometheus.CounterVec
}

// NewMetrics registers the tick's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		unclassified: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: unclassifiedName,
			Help: "The messages delta sync added to the index whose sender the classifier could not classify, which classifies them restricted.",
		}, []string{"account"}),
		backlog: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: backlogName,
			Help: "The account's messages waiting for their content scan, read after each tick's scanning once backfill's second pass has ended.",
		}, []string{"account"}),
		gaps: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: gapsName,
			Help: "The cursor gaps delta sync found, each starting a recovery that re-enumerates the window since the last cursor was written.",
		}, []string{"account"}),
	}
	for _, c := range []prometheus.Collector{m.unclassified, m.backlog, m.gaps} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Count adds what one tick did to the account's series. The unclassified and gap series exist for
// every account a tick served, at zero until a sender goes unclassified or a gap is found, so the
// gap rule sees the first gap as an increase. The backlog series exists only while the tick scans,
// once backfill's second pass has ended, since backfill emits its own before then (ADR-0104).
func (m *Metrics) Count(account string, r Result) {
	m.unclassified.WithLabelValues(account).Add(float64(r.Unclassified))
	gaps := m.gaps.WithLabelValues(account)
	if r.Gap {
		gaps.Inc()
	}
	if r.Scanning {
		m.backlog.WithLabelValues(account).Set(float64(r.Backlog))
	} else {
		m.backlog.DeleteLabelValues(account)
	}
}

// Forget removes the series of an account no tick serves any longer.
func (m *Metrics) Forget(account string) {
	m.unclassified.DeleteLabelValues(account)
	m.backlog.DeleteLabelValues(account)
	m.gaps.DeleteLabelValues(account)
}
