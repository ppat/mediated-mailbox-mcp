// Package scanner is the process's one scanner and the body-processing histogram (ADR-0120,
// ADR-0125, ADR-0142). Get builds the scanner from the scanner part of the program's configuration
// under the revision the configuration library computed for that part, which the process holds, so
// every verdict, mask and serve-time check in the process carries one revision whichever part asks.
// What the scanner decides is core/scan's. Its case is argued in process/README.md.
package scanner

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// key is the top-level key the scanner part sits under, whose revision every verdict records.
const key = "scanner"

// held is the slot the process keeps its one scanner in.
var held = process.NewOnePerProcess[scan.Scanner]("scanner")

// Get returns the process's scanner, built the first time any part asks from c's scanner part under
// the revision of that part the process holds, and the same one on every later call. A part that
// fails its check, or a configuration whose scanner part has no revision, refuses the build.
func Get(ctx context.Context, p process.Capabilities, c config.HasScanner) (scan.Scanner, error) {
	return held.Get(ctx, p, func(_ context.Context, shared process.Capabilities) (scan.Scanner, func(), error) {
		s, err := scan.New(c.ScannerPart(), process.Revision(shared, key))
		return s, nil, err
	})
}

// ProcessingName is the series timing each body's conversion to Markdown and its scan, by step, which
// no rule reads (ADR-0125).
const ProcessingName = "mediated_mailbox_body_processing_duration_seconds"

// The steps ProcessingName times, a body's conversion to Markdown and its scan.
const (
	StepConvert = "convert"
	StepScan    = "scan"
)

// Processing times each body's conversion and scan in the job kind or the operation that registered
// it. A nil Processing times nothing, so a step built without one runs unchanged.
type Processing struct {
	duration *prometheus.HistogramVec
}

// NewProcessing registers on reg the histogram of each body's conversion and scan, labelled by step,
// and returns it. Each job kind or operation that converts and scans registers it on its own
// registerer, so its series carry its own labels.
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
