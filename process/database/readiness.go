package database

import (
	"context"
	"fmt"
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

// ReportReadiness starts the readiness report of the process's one pool, the pool an endpoint, a task
// or a shared thing gets from Get, the database answering a ping, for a program that is ready only
// while it does (ADR-0142). The report is not ready until the first ping answers, ready while pings
// answer, and not ready again from the first that fails. Its loop runs on a goroutine the process
// starts and waits for before it closes the pools, so it never pings a closed pool. It reports under
// the pools' own name, database, built from the process rather than from whoever asked, and is
// started once, however often it is asked for. A job kind's part is refused, since a job kind's pool
// is its own and one name could not tell its report from the process's pool's. A program whose
// readiness does not rest on the database, such as the API server, does not ask for it.
func ReportReadiness(ctx context.Context, p process.Capabilities, c config.HasDatabase) error {
	if kind := process.JobKind(p); kind != "" {
		return fmt.Errorf("the job kind %s asked for a ping report, and a job kind's pool has no ping report yet", kind)
	}
	pool, err := Get(ctx, p, c)
	if err != nil {
		return err
	}
	set, err := held.Get(ctx, p, newPools)
	if err != nil {
		return err
	}
	set.mu.Lock()
	defer set.mu.Unlock()
	if set.reported {
		return nil
	}
	set.reported = true
	set.shared.Report(process.NotReady, "no ping has answered yet")
	set.shared.Go("database ping", func(ctx context.Context) { ping(ctx, pool, set.shared) })
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
