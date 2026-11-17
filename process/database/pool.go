package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// The pool's series, read from the pool's statistics when the registry is gathered (ADR-0125).
const (
	acquiresName         = "mediated_mailbox_db_pool_acquires_total"
	waitedAcquiresName   = "mediated_mailbox_db_pool_waited_acquires_total"
	acquireWaitName      = "mediated_mailbox_db_pool_acquire_wait_seconds_total"
	newConnectionsName   = "mediated_mailbox_db_pool_new_connections_total"
	canceledAcquiresName = "mediated_mailbox_db_pool_canceled_acquires_total"
	acquiredName         = "mediated_mailbox_db_pool_acquired_connections"
	maxName              = "mediated_mailbox_db_pool_max_connections"
)

// OpenPool opens a pool from config with every statement its connections run timed by the tracer
// and its statistics collected, both on reg. It opens no connection until the pool's first use, as
// pgxpool.NewWithConfig does, and runs no schema check. Open opens through it, and it is exported
// only for this package's own tests, which open a pool from a configuration of their own, until
// process/host is deleted and Open with it (ADR-0125).
func OpenPool(ctx context.Context, config *pgxpool.Config, reg prometheus.Registerer) (*pgxpool.Pool, error) {
	tracer, err := newTracer(reg)
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := registerPool(reg, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// unchecked registers a collector the registry does not check, since its Describe yields nothing,
// and collects what the collector it holds collects. Each pool registers its series through it, the
// statement histogram and the pool's statistics, so a job kind's pool, whose series carry job_kind,
// and a pool that is no job kind's, whose series carry none, share one registry: the registry
// compares label names only between checked collectors, and a scrape does not compare them within
// a family (ADR-0125). The cost is the registry's check for a collector registered twice, which it
// makes only for a checked collector. A pool Get or ForMigration gives is opened once, under the
// pools' lock, and registers once. A pool the host opens through Open or OpenPool is opened once by
// its root, until the host is deleted. A scrape that met two series of one name and one set of label
// values would fail, which the tests of a process holding both kinds of pool scrape for.
type unchecked struct {
	prometheus.Collector
}

// Describe yields nothing, which makes the collector unchecked.
func (unchecked) Describe(chan<- *prometheus.Desc) {}

// poolStatistics is a collector over one connection pool. It reads the pool only when gathered, so it adds
// nothing to a statement's path.
type poolStatistics struct {
	pool                                                       *pgxpool.Pool
	acquires, waited, waitSeconds, built, canceled, held, most *prometheus.Desc
}

var _ prometheus.Collector = (*poolStatistics)(nil)

// registerPool registers on reg the collector over pool's statistics. An acquire that builds a new
// connection counts as waited, with its connect time, so the connections built tell those waits
// from waits at the pool's ceiling.
func registerPool(reg prometheus.Registerer, pool *pgxpool.Pool) error {
	desc := func(name, help string) *prometheus.Desc { return prometheus.NewDesc(name, help, nil, nil) }
	return reg.Register(unchecked{&poolStatistics{
		pool:        pool,
		acquires:    desc(acquiresName, "Connections acquired from the pool."),
		waited:      desc(waitedAcquiresName, "Acquires that found no idle connection and waited, for one to be released or for a new one to be built."),
		waitSeconds: desc(acquireWaitName, "The time the acquires that waited for a connection waited, a new connection's connect time included, in seconds."),
		built:       desc(newConnectionsName, "Connections the pool built."),
		canceled:    desc(canceledAcquiresName, "Acquires whose caller gave up waiting for a connection."),
		held:        desc(acquiredName, "Connections held from the pool now."),
		most:        desc(maxName, "The most connections the pool opens."),
	}})
}

// Describe implements prometheus.Collector.
func (p *poolStatistics) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{p.acquires, p.waited, p.waitSeconds, p.built, p.canceled, p.held, p.most} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (p *poolStatistics) Collect(ch chan<- prometheus.Metric) {
	s := p.pool.Stat()
	ch <- prometheus.MustNewConstMetric(p.acquires, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.waited, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.waitSeconds, prometheus.CounterValue, s.EmptyAcquireWaitTime().Seconds())
	ch <- prometheus.MustNewConstMetric(p.built, prometheus.CounterValue, float64(s.NewConnsCount()))
	ch <- prometheus.MustNewConstMetric(p.canceled, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(p.held, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(p.most, prometheus.GaugeValue, float64(s.MaxConns()))
}
