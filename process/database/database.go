// Package database is every pool a process holds (ADR-0142). It builds the connection a program's
// database part describes, opens each pool over it with every statement timed and the pool's
// statistics collected, and runs the schema check on each pool a part asks for through Get before
// it returns it (ADR-0078, ADR-0079, ADR-0125, ADR-0140). Its case is argued in process/README.md.
//
// A job kind's Build gets the job kind's own pool, built for it and labelled with it, so one job
// kind cannot starve another of connections (ADR-0119). An endpoint, a task, or a thing the process
// shares gets the process's one pool, labelled with the program alone. Every pool is opened from the
// one database part and so connects as the one user it names (ADR-0143). The migrator's task gets its
// pool through ForMigration, which runs no schema check and which only the migrator's configuration
// type can call.
//
// A program that is ready only while the database answers opts the process's one pool into the
// readiness report the process answers its readiness probe from, through ReportReadiness, which pings
// the pool every five seconds, each ping bounded by two. A job kind's pool has no such report yet.
//
// The pools of a process are kept in one slot of the process, apart from what other owners keep,
// and the process closes them with what the slots hold, last built first, once its servers have
// drained and its jobs have ended.
package database

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/db/schemacompatibility"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// Get returns the pool of the part p was handed to, opened from c's database part the first time it
// is asked for and kept in the process. A job kind's part gets the job kind's own pool, and every
// other part the process's one pool. The schema check runs on the pool before Get first returns it,
// so a schema the program's chain cannot run on refuses the build, and with it the start, before
// anything serves (ADR-0140).
func Get(ctx context.Context, p process.Capabilities, c config.HasDatabase) (*pgxpool.Pool, error) {
	set, err := held.Get(ctx, p, newPools)
	if err != nil {
		return nil, err
	}
	// A job kind's pool registers its series through the job kind's registerer, so they carry
	// job_kind, and every other pool through the process's own, so they carry none (ADR-0125).
	kind := process.JobKind(p)
	reg := set.shared.Metrics()
	if kind != "" {
		reg = p.Metrics()
	}
	return set.open(ctx, "job kind "+kind, c.DatabasePart(), reg, CheckSchema)
}

// ForMigration returns the migrator's pool, opened from its database part with no schema check, since
// the migrator moves the schema rather than running on it (ADR-0139, ADR-0140). Only the migrator's
// configuration type satisfies config.HasMigratorDatabase, so no deployable's Build can call it with
// its own configuration (ADR-0142).
func ForMigration(ctx context.Context, p process.Capabilities, c config.HasMigratorDatabase) (*pgxpool.Pool, error) {
	set, err := held.Get(ctx, p, newPools)
	if err != nil {
		return nil, err
	}
	return set.open(ctx, "migration", c.DatabasePart(), set.shared.Metrics(), nil)
}

// Open opens a pool from c, connecting as c's user, with every statement it runs timed and its
// statistics collected on reg. It opens no connection until the pool's first use and runs no schema
// check. Get and ForMigration open through it. It is exported only for process/host, which opens its
// pools through it and runs its root's schema check itself, until every program is on its spec and
// the host is deleted, so a deployable's Build calling it is left to review until then.
func Open(ctx context.Context, c config.Database, reg prometheus.Registerer) (*pgxpool.Pool, error) {
	conf, err := poolConfig(c)
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", c.User, err)
	}
	pool, err := OpenPool(ctx, conf, reg)
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", c.User, err)
	}
	return pool, nil
}

// CheckSchema refuses a schema the binary's migration chain cannot run on, read through pool under
// the user pool connects as (ADR-0140).
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	v, err := schemacompatibility.New(pool).SchemaVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading the schema's versions: %w", err)
	}
	return migrations.Refuse(v.Applied, v.OldestChainVersion)
}

// held is the slot the process keeps its pools in.
var held = process.NewOnePerProcess[*pools]("database")

// pools are the pools one process holds, each opened once under the set's lock, by the name of what
// it serves.
type pools struct {
	shared process.Capabilities
	mu     sync.Mutex
	byName map[string]*pgxpool.Pool
	opened []*pgxpool.Pool
	// reported is whether the process's pool's readiness report has started.
	reported bool
}

func newPools(_ context.Context, shared process.Capabilities) (*pools, func(), error) {
	s := &pools{shared: shared, byName: map[string]*pgxpool.Pool{}}
	return s, s.close, nil
}

// open returns the pool kept under name, opening it from c measured on reg the first time and
// running check on it before keeping it. A pool check refuses is closed and not kept.
func (s *pools) open(ctx context.Context, name string, c config.Database, reg prometheus.Registerer, check func(context.Context, *pgxpool.Pool) error) (*pgxpool.Pool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pool, ok := s.byName[name]; ok {
		return pool, nil
	}
	pool, err := Open(ctx, c, reg)
	if err != nil {
		return nil, err
	}
	if check != nil {
		if err := check(ctx, pool); err != nil {
			pool.Close()
			return nil, fmt.Errorf("checking the database schema as %s: %w", c.User, err)
		}
	}
	s.byName[name] = pool
	s.opened = append(s.opened, pool)
	return pool, nil
}

// close closes every pool, last opened first.
func (s *pools) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pool := range slices.Backward(s.opened) {
		pool.Close()
	}
	s.opened, s.byName = nil, map[string]*pgxpool.Pool{}
}
