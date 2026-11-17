package reads

import (
	"github.com/prometheus/client_golang/prometheus"
)

// The UI's own metrics (docs/UI.md section 18.2). No label ever carries message-derived text. The
// dataset and level labels take only values the registry declares, because the dataset endpoint
// refuses every other before it reads.
const (
	readDurationName = "mediated_mailbox_ui_read_duration_seconds"
	subscribersName  = "mediated_mailbox_ui_stream_subscribers"
)

type metrics struct {
	readDuration *prometheus.HistogramVec
	subscribers  prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
	m := &metrics{
		readDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    readDurationName,
			Help:    "Seconds the dataset endpoint took to answer a read, by dataset and level.",
			Buckets: prometheus.DefBuckets,
		}, []string{"dataset", "level"}),
		subscribers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: subscribersName,
			Help: "Event streams open now.",
		}),
	}
	for _, c := range []prometheus.Collector{m.readDuration, m.subscribers} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}
