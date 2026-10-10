// Package series defines the series more than one job kind of the worker emits about the index and
// the bodies it scans, so each has one name, one help text and one set of labels whichever job kind
// emits it. Each job kind registers them on its own registerer, which labels every series it registers
// with the job kind (ADR-0076, ADR-0117, ADR-0125). A series named twice in one process with two help texts would refuse the
// worker's start, which one definition here rules out.
package series

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// The names of the series. ADR-0093 and O2 watch the backlog, and the unclassified count is O2's.
const (
	UnclassifiedName = "mediated_mailbox_unclassified_senders_total"
	BacklogName      = "mediated_mailbox_scan_backlog"
)

// NewUnclassified registers on reg the counter of the messages a job kind made durable in the index
// whose sender the classifier could not classify, labelled by account, and returns it.
func NewUnclassified(reg prometheus.Registerer) (*prometheus.CounterVec, error) {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: UnclassifiedName,
		Help: "The messages the job kind made durable in the index whose sender the classifier could not classify, which classifies them restricted.",
	}, []string{"account"})
	if err := reg.Register(c); err != nil {
		return nil, err
	}
	return c, nil
}

// NewBacklog registers on reg the gauge of the account's messages waiting for their content scan, as
// the job kind last read it, labelled by account, and returns it. A growing backlog turns messages
// into body denials that read like permission failures (ADR-0093).
func NewBacklog(reg prometheus.Registerer) (*prometheus.GaugeVec, error) {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: BacklogName,
		Help: "The account's messages waiting for their content scan, as the job kind last read them after a step that scans.",
	}, []string{"account"})
	if err := reg.Register(g); err != nil {
		return nil, err
	}
	return g, nil
}

// The names of the throughput and body-cost series, which no rule reads (ADR-0125).
const (
	MessagesName   = "mediated_mailbox_index_messages_total"
	ProcessingName = "mediated_mailbox_body_processing_duration_seconds"
)

// The stages MessagesName counts. Indexed is a message a step added to the index, and Scanned a
// message a step scanned and made durable.
const (
	StageIndexed = "indexed"
	StageScanned = "scanned"
)

// The steps ProcessingName times, a body's conversion to Markdown and its scan.
const (
	StepConvert = "convert"
	StepScan    = "scan"
)

// Messages counts the messages a job kind made durable, by account and stage. Its rate is each
// pass's and each tick's throughput. A nil Messages counts nothing, so a step built without one runs
// unchanged.
type Messages struct {
	count *prometheus.CounterVec
}

// NewMessages registers on reg the counter of the messages a job kind made durable, labelled by
// account and stage, and returns it. A job kind registers it once and hands it to each of its steps.
func NewMessages(reg prometheus.Registerer) (*Messages, error) {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MessagesName,
		Help: "The messages the job kind made durable, by stage, indexed for a message it added to the index and scanned for one it scanned.",
	}, []string{"account", "stage"})
	if err := reg.Register(c); err != nil {
		return nil, err
	}
	return &Messages{count: c}, nil
}

// Add adds n messages of the account made durable at stage.
func (m *Messages) Add(account, stage string, n int) {
	if m != nil {
		m.count.WithLabelValues(account, stage).Add(float64(n))
	}
}

// Forget removes the account's series, once no step serves the account any longer.
func (m *Messages) Forget(account string) {
	if m != nil {
		m.count.DeletePartialMatch(prometheus.Labels{"account": account})
	}
}

// Processing times each body's conversion and scan in a job kind. A nil Processing times nothing, so
// a step built without one runs unchanged.
type Processing struct {
	duration *prometheus.HistogramVec
}

// NewProcessing registers on reg the histogram of each body's conversion and scan, labelled by step,
// and returns it.
func NewProcessing(reg prometheus.Registerer) (*Processing, error) {
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    ProcessingName,
		Help:    "How long each body's conversion to Markdown and its scan took, by step.",
		Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"step"})
	if err := reg.Register(h); err != nil {
		return nil, err
	}
	return &Processing{duration: h}, nil
}

// Since observes a step that started at start.
func (p *Processing) Since(step string, start time.Time) {
	if p != nil {
		p.duration.WithLabelValues(step).Observe(time.Since(start).Seconds())
	}
}
