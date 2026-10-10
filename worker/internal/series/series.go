// Package series defines the series more than one job kind of the worker emits about the index, so
// each has one name, one help text and one set of labels whichever job kind emits it. Each job kind
// registers them on its own registerer, which labels every series it registers with the job kind
// (ADR-0076, ADR-0117). A series named twice in one process with two help texts would refuse the
// worker's start, which one definition here rules out.
package series

import "github.com/prometheus/client_golang/prometheus"

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
