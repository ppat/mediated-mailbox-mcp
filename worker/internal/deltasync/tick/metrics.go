package tick

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// gapsName is the series counting the cursor gaps a tick found, delta sync's own concern. The cursor
// gap rule in packaging/chart/alerting-rules.yaml reads it, so it stays exactly as written (ADR-0077,
// ADR-0105). The unclassified and backlog series are the ones backfill also emits, defined once in
// worker/internal/series.
const gapsName = "mediated_mailbox_sync_cursor_gaps_total"

// Metrics are the tick's series on delta sync's registerer in the worker, which runs until stopped, so
// each keeps its value between ticks (ADR-0103).
type Metrics struct {
	unclassified *prometheus.CounterVec
	backlog      *prometheus.GaugeVec
	gaps         *prometheus.CounterVec
}

// NewMetrics registers the tick's series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	unclassified, err := series.NewUnclassified(reg)
	if err != nil {
		return nil, err
	}
	backlog, err := series.NewBacklog(reg)
	if err != nil {
		return nil, err
	}
	m := &Metrics{
		unclassified: unclassified,
		backlog:      backlog,
		gaps: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: gapsName,
			Help: "The cursor gaps delta sync found, each starting a recovery that re-enumerates the window since the last cursor was written.",
		}, []string{"account"}),
	}
	if err := reg.Register(m.gaps); err != nil {
		return nil, err
	}
	return m, nil
}

// Count adds what one tick did to the account's series. The unclassified and gap series exist for
// every account a tick served, at zero until a sender goes unclassified or a gap is found, so the
// gap rule sees the first gap as an increase. The backlog series exists only while the tick scans,
// once backfill's second pass has ended, since backfill's job kind emits the account's own until then
// (ADR-0104).
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
