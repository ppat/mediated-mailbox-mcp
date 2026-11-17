package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// The readiness report's bounds. The pool is pinged every pingInterval, and a ping that has not
// answered within pingTimeout counts as failed, so the report is never older than the two together.
const (
	pingInterval = 5 * time.Second
	pingTimeout  = 2 * time.Second
)

// ReportReadiness starts the readiness report of the pool the part p was handed to gets from Get, the
// database answering a ping, for a program that is ready only while it does (ADR-0142). For an
// endpoint, a task or a shared thing that is the process's one pool, reported under the pools' own
// name, database, built from the process rather than from whoever asked. For a job kind it is the job
// kind's own pool, reported under the job kind's own name, so each job kind's pool is told apart from
// the others and from the process's, as the worker needs, which is ready only while each job kind's
// pool answers. A report is not ready until the first ping answers, ready while pings answer, and not
// ready again from the first that fails. Its loop runs on a goroutine the process starts and waits for
// before it closes the pools, so it never pings a closed pool. Each pool's report is started once,
// however often it is asked for. A program whose readiness does not rest on the database, such as the
// API server, does not ask for it.
func ReportReadiness(ctx context.Context, p process.Capabilities, c config.HasDatabase) error {
	pool, err := Get(ctx, p, c)
	if err != nil {
		return err
	}
	set, err := held.Get(ctx, p, newPools)
	if err != nil {
		return err
	}
	kind := process.JobKind(p)
	reporter := set.shared
	if kind != "" {
		reporter = p
	}
	set.mu.Lock()
	defer set.mu.Unlock()
	if set.reported[kind] {
		return nil
	}
	set.reported[kind] = true
	reporter.Report(process.NotReady, "no ping has answered yet")
	reporter.Go("database ping", func(ctx context.Context) { ping(ctx, pool, reporter) })
	return nil
}

// ping pings pool every pingInterval until ctx ends, and reports each answer.
func ping(ctx context.Context, pool *pgxpool.Pool, reporter process.Capabilities) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		pinging, cancel := context.WithTimeout(ctx, pingTimeout)
		err := pool.Ping(pinging)
		cancel()
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			reporter.Logger().DebugContext(ctx, "the database did not answer a ping", "error", err)
			reporter.Report(process.NotReady, "the database does not answer a ping")
		default:
			reporter.Report(process.Ready, "")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
