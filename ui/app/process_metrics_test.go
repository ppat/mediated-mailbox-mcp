package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
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

// The pool the composition root opens traces every statement it runs and is read on the registry its
// metrics endpoint serves, and it opens no connection to be read (ADR-0125).
func TestThePoolIsMeasured(t *testing.T) {
	password := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(password, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	pool, err := openPool(t.Context(), dbconnectcore.Config{
		Host: "db.invalid", Port: 5432, Name: "mailbox", User: "mediated_mailbox", SSLMode: "verify-full", PasswordFile: password,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, traced := pool.Config().ConnConfig.Tracer.(*dbmetrics.Tracer); !traced {
		t.Error("the pool traces no statement")
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var ceiling []float64
	for _, f := range families {
		if f.GetName() == "mediated_mailbox_db_pool_max_connections" {
			for _, m := range f.GetMetric() {
				ceiling = append(ceiling, m.GetGauge().GetValue())
			}
		}
	}
	if want := []float64{float64(pool.Config().MaxConns)}; !slices.Equal(ceiling, want) {
		t.Errorf("the pool's ceiling reads %v, want %v", ceiling, want)
	}
}
