package process

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// addedRuntimeMetrics names the runtime metrics added to the Go collector's default set, each by its
// whole name: the live heap, the garbage collector's and the process's CPU time, and the scheduling
// latency.
var addedRuntimeMetrics = regexp.MustCompile(`^(/gc/heap/live:bytes|/cpu/classes/gc/total:cpu-seconds|/cpu/classes/total:cpu-seconds|/sched/latencies:seconds)$`)

// registerRuntimeMetrics registers on reg the Go runtime's and the process's series, the Go
// collector, its default set with the added runtime metrics, and the process collector, once per
// process, so the set of runtime metrics is named in one place (ADR-0125). reg is the process's own
// registry, never one a part labels, since these series belong to the process.
func registerRuntimeMetrics(reg prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: addedRuntimeMetrics})),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	} {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}
