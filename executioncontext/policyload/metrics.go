package policyload

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// reloadFailedName is the series the reload-failure alarm reads, in packaging/chart/alerting-rules.yaml.
//
// It is a gauge of the latest reload's outcome rather than a counter of failures. A process loads
// policy first as it starts, usually before its first scrape, and a counter whose first sample is
// already 1 shows no increase, so a rule over a counter would never see that failure.
const reloadFailedName = "mediated_mailbox_policyload_reload_failed"

// reloadDurationName is the series timing each reload its caller did not cancel, which no rule
// reads. It answers how much of a body request the policy load it waits for takes (ADR-0099,
// ADR-0125).
const reloadDurationName = "mediated_mailbox_policyload_reload_duration_seconds"

// metrics is one process's reload series (ADR-0076, ADR-0077, ADR-0125).
type metrics struct {
	reloadFailed   prometheus.Gauge
	reloadDuration prometheus.Histogram
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
	m := &metrics{
		reloadFailed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: reloadFailedName,
			Help: "1 while this process's latest policy reload failed, in its read of the policy tables or in their validation, and 0 once a reload succeeds.",
		}),
		reloadDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    reloadDurationName,
			Help:    "How long each policy reload its caller did not cancel took, succeeded or failed, from its call to its outcome, its wait for its turn included.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}),
	}
	for _, c := range []prometheus.Collector{m.reloadFailed, m.reloadDuration} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// took observes a reload called at start.
func (m *metrics) took(start time.Time) { m.reloadDuration.Observe(time.Since(start).Seconds()) }

func (m *metrics) failed() { m.reloadFailed.Set(1) }

func (m *metrics) succeeded() { m.reloadFailed.Set(0) }
