// Package runtimemetrics registers the Go runtime's and the process's series on a process's registry,
// once per process (ADR-0125). Each composition root calls it, so the set of runtime metrics is named
// in one place. Its case is argued in process/README.md.
package runtimemetrics

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// added names the runtime metrics added to the Go collector's default set, each by its whole name:
// the live heap, the garbage collector's and the process's CPU time, and the scheduling latency.
var added = regexp.MustCompile(`^(/gc/heap/live:bytes|/cpu/classes/gc/total:cpu-seconds|/cpu/classes/total:cpu-seconds|/sched/latencies:seconds)$`)

// Register registers on reg the Go collector, its default set with the added runtime metrics, and
// the process collector. reg is the process's own registry, never one a job kind labels, since these
// series belong to the process.
func Register(reg prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: added})),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	} {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}
