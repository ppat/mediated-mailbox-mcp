package policyload

import "github.com/prometheus/client_golang/prometheus"

// reloadFailedName is the series the reload-failure alarm reads, in packaging/chart/alerting-rules.yaml.
//
// It is a gauge of the latest reload's outcome rather than a counter of failures. A process loads
// policy first as it starts, usually before its first scrape, and a counter whose first sample is
// already 1 shows no increase, so a rule over a counter would never see that failure.
const reloadFailedName = "mediated_mailbox_policyload_reload_failed"

// metrics is one process's reload-failure series (ADR-0076, ADR-0077).
type metrics struct {
	reloadFailed prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
	m := &metrics{reloadFailed: prometheus.NewGauge(prometheus.GaugeOpts{
		Name: reloadFailedName,
		Help: "1 while this process's latest policy reload failed, in its read of the policy tables or in their validation, and 0 once a reload succeeds.",
	})}
	if err := reg.Register(m.reloadFailed); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *metrics) failed() { m.reloadFailed.Set(1) }

func (m *metrics) succeeded() { m.reloadFailed.Set(0) }
