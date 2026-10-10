package app

import (
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// The process carries the Go collector's default set with the four runtime metrics added to it, the
// live heap, the garbage collector's and the process's CPU time and the scheduling latency, and the
// process collector's series, and no other runtime metric (ADR-0125).
func TestTheRuntimeAndProcessSeriesAreRegistered(t *testing.T) {
	registry := prometheus.NewRegistry()
	if err := registerProcess(registry); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range families {
		names = append(names, f.GetName())
	}
	for _, want := range []string{
		"go_goroutines", "go_gc_gogc_percent", "go_gc_gomemlimit_bytes", "go_sched_gomaxprocs_threads",
		"go_gc_heap_live_bytes", "go_cpu_classes_gc_total_cpu_seconds_total", "go_cpu_classes_total_cpu_seconds_total",
		"go_sched_latencies_seconds", "process_resident_memory_bytes", "process_start_time_seconds", "process_open_fds",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("the registry serves no %s", want)
		}
	}
	for _, unwanted := range []string{"go_sync_mutex_wait_total_seconds_total", "go_gc_heap_allocs_by_size_bytes", "go_cpu_classes_idle_cpu_seconds_total"} {
		if slices.Contains(names, unwanted) {
			t.Errorf("the registry serves %s, a runtime metric the set leaves out", unwanted)
		}
	}
}
