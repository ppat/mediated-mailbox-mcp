package dbmetrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// The pool's series, read from the pool's statistics when the registry is gathered (ADR-0125).
const (
	acquiresName         = "mediated_mailbox_db_pool_acquires_total"
	waitedAcquiresName   = "mediated_mailbox_db_pool_waited_acquires_total"
	acquireWaitName      = "mediated_mailbox_db_pool_acquire_wait_seconds_total"
	canceledAcquiresName = "mediated_mailbox_db_pool_canceled_acquires_total"
	acquiredName         = "mediated_mailbox_db_pool_acquired_connections"
	maxName              = "mediated_mailbox_db_pool_max_connections"
)

// Pool is a collector over one connection pool. It reads the pool only when gathered, so it adds
// nothing to a statement's path.
type Pool struct {
	pool                                                *pgxpool.Pool
	acquires, waited, waitSeconds, canceled, held, most *prometheus.Desc
}

var _ prometheus.Collector = (*Pool)(nil)

// RegisterPool registers on reg the collector over pool's statistics.
func RegisterPool(reg prometheus.Registerer, pool *pgxpool.Pool) error {
	desc := func(name, help string) *prometheus.Desc { return prometheus.NewDesc(name, help, nil, nil) }
	return reg.Register(&Pool{
		pool:        pool,
		acquires:    desc(acquiresName, "Connections acquired from the pool."),
		waited:      desc(waitedAcquiresName, "Acquires that waited for a connection because none was idle."),
		waitSeconds: desc(acquireWaitName, "The time the acquires that waited for a connection waited, in seconds."),
		canceled:    desc(canceledAcquiresName, "Acquires whose caller gave up waiting for a connection."),
		held:        desc(acquiredName, "Connections held from the pool now."),
		most:        desc(maxName, "The most connections the pool opens."),
	})
}

// Describe implements prometheus.Collector.
func (p *Pool) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{p.acquires, p.waited, p.waitSeconds, p.canceled, p.held, p.most} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (p *Pool) Collect(ch chan<- prometheus.Metric) {
	s := p.pool.Stat()
	ch <- prometheus.MustNewConstMetric(p.acquires, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.waited, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.waitSeconds, prometheus.CounterValue, s.EmptyAcquireWaitTime().Seconds())
	ch <- prometheus.MustNewConstMetric(p.canceled, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.held, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(p.most, prometheus.GaugeValue, float64(s.MaxConns()))
}
